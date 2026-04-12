// mk-go 用の Cypress support ファイル。
// upstream Misskey の support を読み込んだうえで、mk-go 固有の例外抑制を追加する。

import '../../../third_party/misskey/cypress/support/e2e';

// MkModal.vue の上流バグ: content.value.children[0] が undefined のまま
// addEventListener を呼ぶ。上流 (misskey-dev/misskey) が修正するまで抑制する。
// See: https://github.com/shiroha-a/mk/issues/39
Cypress.on('uncaught:exception', (err) => {
	if (err.message.includes("Cannot read properties of undefined (reading 'addEventListener')")) {
		return false;
	}
});
