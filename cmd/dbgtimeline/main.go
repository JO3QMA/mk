// Command dbgtimeline is a debug helper that reproduces (or hunts for)
// JSON-encoder panics on the home/global timeline path. It loads recent
// notes from a live PostgreSQL DB, packs them through `entity.PackNotes`
// + `entity.NoteFieldResolver` exactly as the timeline endpoint does,
// then attempts to encode the result via goccy/go-json — recovering each
// panic so the offending note can be identified.
//
// Originally written to track down the goccy 0.10.6 ptrToString crash on
// remote Renote-with-Files notes (see cmd/dbgtimeline/README.md and #542).
// Kept around as a generic debug tool: any future "encoder panics on
// timeline" report can be triaged with this same flow.
//
// Read-only: only SELECT queries via gorm.Find / repository read methods
// are issued. Safe to point at a production-like DB. Never writes.
//
// See cmd/dbgtimeline/README.md for invocation examples.
package main

import (
	stdjson "encoding/json"
	"flag"
	"fmt"
	"os"

	gojson "github.com/goccy/go-json"
	"github.com/shiroha-a/mk/internal/config"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfgPath := flag.String("config", "/app/.config/default.yml", "path to mk-go config file")
	limit := flag.Int("limit", 200, "max notes to scan")
	viewerName := flag.String("viewer", "", "username (lowercased) used to populate viewer-dependent fields like MyReaction; empty = nil viewer")
	bisect := flag.Bool("bisect", false, "binary-search the slice to narrow down the panic-triggering index range")
	dump := flag.Bool("dump", false, "deep-dump packed entity for the first panicking note (stdlib JSON, never touches goccy)")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		die("load config: %v", err)
	}
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Pass, cfg.DB.DB)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		die("open db: %v", err)
	}
	idType := cfg.ID
	if idType == "" {
		idType = "aidx"
	}
	idGen, err := id.NewGenerator(idType)
	if err != nil {
		die("idGen (%q): %v", idType, err)
	}

	noteRepo := repository.NewNoteRepository(db)
	userRepo := repository.NewUserRepository(db)
	instanceRepo := repository.NewInstanceRepository(db)
	emojiRepo := repository.NewEmojiRepository(db)
	driveFileRepo := repository.NewDriveFileRepository(db)
	driveFolderRepo := repository.NewDriveFolderRepository(db)
	noteReactionRepo := repository.NewNoteReactionRepository(db)
	channelRepo := repository.NewChannelRepository(db)

	// 全 visibility / 全 host を含む幅広いサンプル。実際の global timeline は
	// public + channel=null だが、bug は home (followers / specified) でも
	// 出る報告があるので幅広く取る。read-only な Find のみ。
	var notes []*model.Note
	if err := db.Order(`"id" DESC`).Limit(*limit).Find(&notes).Error; err != nil {
		die("load notes: %v", err)
	}

	// repo 経由で hydrate して timeline 経路と同じ preload chain を再現する
	// (NoteFieldResolver.Apply は preload された Renote.User 等を頼る)。
	hydrated := make([]*model.Note, 0, len(notes))
	for _, n := range notes {
		h, err := noteRepo.FindByIDWithRelations(n.ID)
		if err != nil || h == nil {
			continue
		}
		hydrated = append(hydrated, h)
	}
	notes = hydrated
	fmt.Printf("Loaded %d notes\n", len(notes))

	packed := entity.PackNotes(notes, idGen, instanceRepo, emojiRepo)
	// PackNotes は notes 1 件ごとに必ず 1 件の packed entity を append する
	// 契約 (内部実装は internal/entity/note.go の PackNotes を参照)。後続
	// ループで notes[i] と packed[i] を 1:1 対応で参照するため、契約破綻時
	// は静かに index ずれを起こすより die で早期検出する (#545 review)。
	if len(packed) != len(notes) {
		die("invariant broken: len(packed)=%d != len(notes)=%d (PackNotes filtered entries?)", len(packed), len(notes))
	}
	resolver := entity.NewNoteFieldResolver(driveFileRepo, driveFolderRepo, userRepo, noteReactionRepo, channelRepo, idGen)

	var viewer *model.User
	if *viewerName != "" {
		if u, err := userRepo.FindByUsernameLower(*viewerName, nil); err == nil {
			viewer = u
			fmt.Printf("Using viewer: %s (id=%s)\n", u.Username, u.ID)
		} else {
			fmt.Printf("Viewer %q not found, encoding with nil viewer\n", *viewerName)
		}
	}
	resolver.Apply(packed, viewer)

	// Per-note encode で panic 起こす index を列挙。tryEncode は recover する
	// が、goccy の VM 状態が後続呼び出しで悪化することがあるので fatal で
	// プロセス死ぬケースに備えて発見都度 print する。
	failed := []int{}
	for i := range packed {
		one := packed[i : i+1]
		if err := tryEncode(one); err != nil {
			failed = append(failed, i)
			fmt.Printf("PANIC at index=%d note.id=%s userHost=%s visibility=%s renoteId=%s\n",
				i, notes[i].ID, derefStr(notes[i].UserHost), notes[i].Visibility, derefStr(notes[i].RenoteID))
		}
	}
	if len(failed) == 0 {
		fmt.Println("No panic detected on per-note encode")
	} else {
		fmt.Printf("Total panicking notes: %d\n", len(failed))
	}

	if *bisect && len(failed) > 0 {
		fmt.Println("\n--- Slice bisect (find smallest panicking range) ---")
		bisectSlice(packed, 0, len(packed))
	}

	if *dump && len(failed) > 0 {
		fmt.Println("\n--- Deep dump of first panicking note (stdlib) ---")
		fmt.Println(spew(packed[failed[0]]))
	}
}

// tryEncode runs gojson.Encode on v (matching the production
// json_serializer.go fast path) and converts panics to errors so the
// caller can keep iterating across many candidates. NOTE: goccy panics
// occasionally escalate to runtime.fatal which recover() cannot catch —
// the caller should be prepared for the process to die mid-loop.
func tryEncode(v any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return gojson.NewEncoder(devNull{}).Encode(v)
}

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }

// bisectSlice narrows down which contiguous range of `s` triggers a
// goccy panic. Recurses into halves that fail. Single-element ranges
// are reported with their note id for further investigation.
func bisectSlice(s []entity.NoteEntity, lo, hi int) {
	if hi-lo <= 1 {
		one := s[lo : lo+1]
		err := tryEncode(one)
		fmt.Printf("[single index=%d] err=%v id=%s\n", lo, err, s[lo].ID)
		return
	}
	mid := (lo + hi) / 2
	leftErr := tryEncode(s[lo:mid])
	rightErr := tryEncode(s[mid:hi])
	fmt.Printf("[bisect lo=%d mid=%d hi=%d] left err=%v / right err=%v\n", lo, mid, hi, leftErr, rightErr)
	if leftErr != nil {
		bisectSlice(s, lo, mid)
	}
	if rightErr != nil {
		bisectSlice(s, mid, hi)
	}
}

// spew pretty-prints v via the standard library encoder. Intentionally
// avoids goccy so the dump is safe even when goccy is what's panicking.
func spew(v any) string {
	b, err := stdjson.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("ERR: %v", err)
	}
	return string(b)
}

func derefStr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
