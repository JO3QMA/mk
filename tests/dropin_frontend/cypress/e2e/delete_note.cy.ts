// Phase 14-2 (#387) ノート削除の federation 伝播検証。
//
// charlie が public note を投稿 → alice / bob の home に届く → charlie が削除
// → alice / bob の timeline からも消える、を確認。
//
// Misskey TS 同士なら baseline として通るべき挙動。Phase 14-3 で mk-A 切替時
// に #379 (削除済ノートが TL に残存) が regression として検出される予定。

import { api, INSTANCES } from '../support/api';
import { establishFederation, setupTrio, waitForNoteInTimeline, Trio } from '../support/setup';

describe('dropin-frontend delete propagation (Phase 14-2)', () => {
  let trio: Trio;

  before(() => {
    setupTrio().then((t) => {
      trio = t;
    });
    cy.then(() => establishFederation(trio));
  });

  it('charlie がノートを削除すると A と B の timeline から消える', () => {
    const marker = `phase14-delete-${Date.now()}`;
    let originalId = '';

    // charlie が投稿
    api(INSTANCES.c, 'notes/create', {
      i: trio.charlie.token,
      text: marker,
      visibility: 'public',
    }).then((resp) => {
      originalId = resp.body.createdNote.id;
    });

    // A / B 両方で一度届くのを確認する (= 削除テストの前提条件)
    cy.then(() => waitForNoteInTimeline(trio.alice, marker, { retries: 40 }));
    cy.then(() => waitForNoteInTimeline(trio.bob, marker, { retries: 40 }));

    // charlie が削除
    cy.then(() =>
      api(INSTANCES.c, 'notes/delete', {
        i: trio.charlie.token,
        noteId: originalId,
      }).then((resp) => {
        expect(resp.status).to.be.oneOf([200, 204]);
      }),
    );

    // A 側から消えるまで poll
    const pollGone = (viewer: typeof trio.alice, inst: typeof INSTANCES.a) =>
      cy.then(() => {
        const attempt = (left: number): Cypress.Chainable => {
          return api(inst, 'notes/timeline', {
            i: viewer.token,
            limit: 40,
          }).then((resp) => {
            const stillThere =
              resp.status === 200 &&
              Array.isArray(resp.body) &&
              resp.body.some((n: Record<string, unknown>) => n.text === marker);
            if (!stillThere) return resp;
            if (left <= 0) {
              throw new Error(
                `note "${marker}" still visible on ${inst.domain} after delete`,
              );
            }
            return cy.wait(3_000, { log: false }).then(() => attempt(left - 1));
          });
        };
        attempt(30);
      });

    pollGone(trio.alice, INSTANCES.a);
    pollGone(trio.bob, INSTANCES.b);
  });
});
