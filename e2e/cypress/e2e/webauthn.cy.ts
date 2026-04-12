/// <reference types="cypress" />

// WebAuthn E2E: Chrome DevTools Protocol の Virtual Authenticator を使い、
// FIDO2 セキュリティキーの登録 → ログインのフルフローを検証する。
// Go 側の FinishLogin success path が実際のブラウザ WebAuthn API 経由で
// 動作することを確認する (issue #55)。

describe('WebAuthn (Virtual Authenticator)', () => {
  const admin = { username: 'admin', password: 'adminpass' };
  const user = { username: 'webauthnuser', password: 'pass123456' };

  // CDP (Chrome DevTools Protocol) でvirtual authenticatorを有効化する。
  // Cypress は Chromium ベースブラウザで CDP コマンドを直接送れる。
  let authenticatorId: string;

  function enableVirtualAuthenticator(): Cypress.Chainable {
    return cy.wrap(
      Cypress.automation('remote:debugger:protocol', {
        command: 'WebAuthn.enable',
        params: {},
      }).then(() =>
        Cypress.automation('remote:debugger:protocol', {
          command: 'WebAuthn.addVirtualAuthenticator',
          params: {
            options: {
              protocol: 'ctap2',
              transport: 'usb',
              hasResidentKey: true,
              hasUserVerification: true,
              isUserVerified: true,
            },
          },
        })
      ).then((result: { authenticatorId: string }) => {
        authenticatorId = result.authenticatorId;
      })
    );
  }

  function disableVirtualAuthenticator(): Cypress.Chainable {
    if (!authenticatorId) return cy.wrap(null);
    return cy.wrap(
      Cypress.automation('remote:debugger:protocol', {
        command: 'WebAuthn.removeVirtualAuthenticator',
        params: { authenticatorId },
      }).then(() =>
        Cypress.automation('remote:debugger:protocol', {
          command: 'WebAuthn.disable',
          params: {},
        })
      )
    );
  }

  before(() => {
    // 初期状態にリセットし admin + テストユーザーを作成する。
    cy.resetState();
    cy.registerUser(admin.username, admin.password, true);
    cy.registerUser(user.username, user.password);
  });

  afterEach(() => {
    disableVirtualAuthenticator();
  });

  it('registers a security key and logs in with it', () => {
    enableVirtualAuthenticator();

    // Step 1: ログインしてトークンを取得
    cy.request('POST', '/api/signin-flow', {
      username: user.username,
      password: user.password,
    }).then((signinRes) => {
      expect(signinRes.body.finished).to.be.true;
      const token = signinRes.body.i;

      // Step 2: 2FA を有効化 (TOTP 登録)
      cy.request({
        method: 'POST',
        url: '/api/i/2fa/register',
        headers: { Authorization: `Bearer ${token}` },
        body: { password: user.password },
      }).then((tfaRes) => {
        // TOTP secret を使って OTP を生成してdoneに渡す (簡易: backupコード経由)
        // ここでは TOTP secret を直接使って検証コードを生成する代わりに、
        // register → done の流れをAPI経由でテストする。
        const secret = tfaRes.body.secret;
        expect(secret).to.be.a('string');

        // TOTP コードを生成 (テスト用に secret から計算)
        // Cypress では Node.js の crypto を使えないため、
        // テスト用のワンタイムパスワード生成はサーバーサイドで行う。
        // 代替: バックアップコードを使わずに、register-key エンドポイントだけをテストする。

        // Step 3: セキュリティキー登録 (register-key → ブラウザ WebAuthn API → key-done)
        cy.request({
          method: 'POST',
          url: '/api/i/2fa/register-key',
          headers: { Authorization: `Bearer ${token}` },
          body: { password: user.password },
        }).then((regKeyRes) => {
          // register-key は WebAuthn credential creation options を返す
          const options = regKeyRes.body;
          expect(options).to.have.property('rp');
          expect(options).to.have.property('challenge');

          // Step 4: ブラウザの WebAuthn API で credential を作成
          // (Virtual Authenticator が自動応答する)
          cy.window().then(async (win) => {
            const publicKey = {
              ...options,
              challenge: Uint8Array.from(atob(options.challenge.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0)),
              user: {
                ...options.user,
                id: Uint8Array.from(atob(options.user.id.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0)),
              },
              ...(options.excludeCredentials ? {
                excludeCredentials: options.excludeCredentials.map((c: any) => ({
                  ...c,
                  id: Uint8Array.from(atob(c.id.replace(/-/g, '+').replace(/_/g, '/')), c2 => c2.charCodeAt(0)),
                })),
              } : {}),
            };

            const credential = await win.navigator.credentials.create({ publicKey }) as PublicKeyCredential;
            expect(credential).to.not.be.null;

            const response = credential.response as AuthenticatorAttestationResponse;
            const attestationObject = btoa(String.fromCharCode(...new Uint8Array(response.attestationObject)));
            const clientDataJSON = btoa(String.fromCharCode(...new Uint8Array(response.clientDataJSON)));
            const rawId = btoa(String.fromCharCode(...new Uint8Array(credential.rawId)));

            // Step 5: key-done に attestation を送信して登録完了
            cy.request({
              method: 'POST',
              url: '/api/i/2fa/key-done',
              headers: { Authorization: `Bearer ${token}` },
              body: {
                clientDataJSON,
                attestationObject,
                password: user.password,
                name: 'test-key',
              },
            }).then((doneRes) => {
              expect(doneRes.status).to.eq(200);

              // Step 6: FinishLogin テスト — セキュリティキーでログイン
              // signin-flow で password を送ると 2FA step に入る
              cy.request('POST', '/api/signin-flow', {
                username: user.username,
                password: user.password,
              }).then((step2Res) => {
                expect(step2Res.body.finished).to.be.false;
                // セキュリティキーがあるので "captcha-keys" が返る
                expect(step2Res.body.next).to.eq('captcha-keys');
                expect(step2Res.body).to.have.property('assertion');
                expect(step2Res.body).to.have.property('sessionId');

                const assertion = step2Res.body.assertion;
                const sessionId = step2Res.body.sessionId;

                // Step 7: ブラウザの WebAuthn API で assertion を取得
                cy.window().then(async (win2) => {
                  const getOptions: PublicKeyCredentialRequestOptions = {
                    challenge: Uint8Array.from(atob(assertion.publicKey.challenge.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0)),
                    rpId: assertion.publicKey.rpId,
                    allowCredentials: (assertion.publicKey.allowCredentials || []).map((c: any) => ({
                      ...c,
                      id: Uint8Array.from(atob(c.id.replace(/-/g, '+').replace(/_/g, '/')), c2 => c2.charCodeAt(0)),
                    })),
                    timeout: assertion.publicKey.timeout,
                    userVerification: assertion.publicKey.userVerification,
                  };

                  const assertionCred = await win2.navigator.credentials.get({ publicKey: getOptions }) as PublicKeyCredential;
                  expect(assertionCred).to.not.be.null;

                  const assertionResponse = assertionCred.response as AuthenticatorAssertionResponse;

                  const credJSON = {
                    id: assertionCred.id,
                    rawId: btoa(String.fromCharCode(...new Uint8Array(assertionCred.rawId))),
                    type: assertionCred.type,
                    response: {
                      authenticatorData: btoa(String.fromCharCode(...new Uint8Array(assertionResponse.authenticatorData))),
                      clientDataJSON: btoa(String.fromCharCode(...new Uint8Array(assertionResponse.clientDataJSON))),
                      signature: btoa(String.fromCharCode(...new Uint8Array(assertionResponse.signature))),
                      userHandle: assertionResponse.userHandle
                        ? btoa(String.fromCharCode(...new Uint8Array(assertionResponse.userHandle)))
                        : null,
                    },
                  };

                  // Step 8: signin-flow にcredentialを送ってログイン完了
                  cy.request('POST', '/api/signin-flow', {
                    username: user.username,
                    password: user.password,
                    credential: credJSON,
                    sessionId,
                  }).then((loginRes) => {
                    // FinishLogin success path がここで検証される
                    expect(loginRes.status).to.eq(200);
                    expect(loginRes.body.finished).to.be.true;
                    expect(loginRes.body).to.have.property('i');
                    expect(loginRes.body.i).to.be.a('string').and.not.be.empty;
                  });
                });
              });
            });
          });
        });
      });
    });
  });
});
