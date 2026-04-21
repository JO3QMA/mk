import { defineConfig } from 'cypress';

// Phase 14-1 (#381) drop-in frontend e2e 向け cypress 設定。
//
// - spec は `e2e/*.cy.ts` にフラットに置く (階層化は Phase 14-2 で検討)
// - baseUrl は設定しない (spec 内で 3 instance の URL を個別に叩く)
// - self-signed cert は `NODE_TLS_REJECT_UNAUTHORIZED=0` を compose で
//   cypress-runner に渡して node 側 (= cy.request の backing runtime) に
//   許容させる。Phase 14-2 でブラウザ navigation を足す時は electron 側の
//   `--ignore-certificate-errors` も別途必要 (現状の cy.request だけなら不要)。

export default defineConfig({
  e2e: {
    specPattern: 'e2e/**/*.cy.ts',
    supportFile: 'support/e2e.ts',

    viewportWidth: 1280,
    viewportHeight: 720,

    // Misskey の起動・連合配信 poll のため緩めに。
    defaultCommandTimeout: 15_000,
    requestTimeout: 30_000,
    responseTimeout: 30_000,
  },
});
