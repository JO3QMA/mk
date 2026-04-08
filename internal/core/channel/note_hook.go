package channel

// NoteCreateHook adapts channel.Service to core/note.ChannelHook so that
// NoteCreateService can validate channel existence and bump counters via the
// service. core/note 側からは小さな interface 経由で参照されるため、ここで
// 型変換 / 単純な転送のみを行う。
type NoteCreateHook struct {
	svc *Service
}

// NewNoteCreateHook constructs a NoteCreateHook bound to the given Service.
func NewNoteCreateHook(svc *Service) *NoteCreateHook {
	return &NoteCreateHook{svc: svc}
}

// EnsureChannelExists implements core/note.ChannelHook.
func (h *NoteCreateHook) EnsureChannelExists(channelID string) error {
	_, err := h.svc.Show(channelID)
	return err
}

// OnNotePosted implements core/note.ChannelHook.
func (h *NoteCreateHook) OnNotePosted(channelID string) {
	h.svc.OnNotePosted(channelID)
}
