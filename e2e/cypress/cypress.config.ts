import { defineConfig } from 'cypress';
import * as path from 'path';

// mk-go 側の Cypress ラッパー。
//
// ライセンス境界のため、Misskey 本家の Cypress 資産 (spec / support / fixtures)
// はすべて git submodule 経由で `third_party/misskey/cypress` 配下にあり、
// mk-go のリポジトリにはコピーされていない。本ファイルはそれらの場所を
// Cypress に教えているだけで、本家コードの再配布には該当しない。
//
// baseUrl は `E2E_BASE_URL` で上書きできる。デフォルトは Makefile で
// `MK_TESTMODE=1 ./built/misskey` を起動したときの port 3000。

const repoRoot = path.resolve(__dirname, '..', '..');
const misskeyCypress = path.join(repoRoot, 'third_party', 'misskey', 'cypress');

export default defineConfig({
  e2e: {
    baseUrl: process.env.E2E_BASE_URL ?? 'http://localhost:3000',

    // Spec / support / fixtures はすべて submodule の現物を指す。
    specPattern: path.join(misskeyCypress, 'e2e', '**', '*.cy.{js,jsx,ts,tsx}'),
    supportFile: path.join(misskeyCypress, 'support', 'e2e.ts'),
    fixturesFolder: path.join(misskeyCypress, 'fixtures'),

    // ブラウザ起動時のデフォルト解像度。本家と揃えておく。
    viewportWidth: 1280,
    viewportHeight: 720,

    // Misskey 本家の初期セットアップでは少し時間がかかるので緩めに取る。
    defaultCommandTimeout: 10_000,
    requestTimeout: 15_000,
    responseTimeout: 30_000,
  },
});
