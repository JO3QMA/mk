/// <reference types="cypress" />

// WebAuthn E2E: Chrome DevTools Protocol の Virtual Authenticator を使い、
// FIDO2 セキュリティキーの登録 → ログインのフルフローを検証する。
// Go 側の FinishLogin success path が実際のブラウザ WebAuthn API 経由で
// 動作することを確認する (issue #55)。

// base64url → Uint8Array
const b64u = (s: string) =>
  Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0));

// Uint8Array → base64url (go-webauthn の URLEncodedBase64 に合わせる)
const toB64url = (buf: ArrayBuffer) =>
  btoa(String.fromCharCode(...new Uint8Array(buf)))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');

describe('WebAuthn (Virtual Authenticator)', () => {
  const admin = { username: 'admin', password: 'adminpass' };
  const user = { username: 'wanuser', password: 'pass123456' };

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
    cy.resetState();
    cy.registerUser(admin.username, admin.password, true);
    cy.get(`@${admin.username}`).then((res: any) => {
      cy.request({
        method: 'POST',
        url: '/api/admin/accounts/create',
        headers: { Authorization: `Bearer ${res.token}` },
        body: { username: user.username, password: user.password },
      });
    });
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
      const authHeader = { Authorization: `Bearer ${token}` };

      // Step 2: セキュリティキー登録 challenge 取得
      // (2FA が有効でなくても register-key は呼べる — key 追加時に自動で有効化)
      cy.request({
        method: 'POST',
        url: '/api/i/2fa/register-key',
        headers: authHeader,
        body: { password: user.password },
      }).then((regKeyRes) => {
        expect(regKeyRes.status).to.eq(200);
        const regSessionId = regKeyRes.body.sessionId;
        const creationOpts = regKeyRes.body.creation.publicKey;
        expect(creationOpts).to.have.property('rp');
        expect(creationOpts).to.have.property('challenge');

        // Step 3: ブラウザの WebAuthn API で credential を作成
        cy.window().then(async (win) => {
          const publicKey = {
            ...creationOpts,
            challenge: b64u(creationOpts.challenge),
            user: {
              ...creationOpts.user,
              id: b64u(creationOpts.user.id),
            },
            ...(creationOpts.excludeCredentials
              ? {
                  excludeCredentials: creationOpts.excludeCredentials.map((c: any) => ({
                    ...c,
                    id: b64u(c.id),
                  })),
                }
              : {}),
          };

          const credential = (await win.navigator.credentials.create({
            publicKey,
          })) as PublicKeyCredential;
          expect(credential).to.not.be.null;

          const attResp = credential.response as AuthenticatorAttestationResponse;

          // Step 4: key-done に attestation を送信して登録完了
          // handler は { password, name, sessionId, response } を期待する。
          // response は go-webauthn が parse する credential creation data。
          cy.request({
            method: 'POST',
            url: '/api/i/2fa/key-done',
            headers: authHeader,
            body: {
              password: user.password,
              name: 'virtual-key',
              sessionId: regSessionId,
              response: {
                id: credential.id,
                rawId: toB64url(credential.rawId),
                type: credential.type,
                response: {
                  clientDataJSON: toB64url(attResp.clientDataJSON),
                  attestationObject: toB64url(attResp.attestationObject),
                },
              },
            },
          }).then((doneRes) => {
            expect(doneRes.status).to.eq(200);

            // Step 5: signin-flow で password → 2FA step
            cy.request('POST', '/api/signin-flow', {
              username: user.username,
              password: user.password,
            }).then((step2Res) => {
              expect(step2Res.body.finished).to.be.false;
              expect(step2Res.body.next).to.eq('captcha-keys');
              expect(step2Res.body).to.have.property('assertion');
              expect(step2Res.body).to.have.property('sessionId');

              const assertion = step2Res.body.assertion;
              const loginSessionId = step2Res.body.sessionId;

              // Step 6: ブラウザの WebAuthn API で assertion を取得
              cy.window().then(async (win2) => {
                const getOpts: PublicKeyCredentialRequestOptions = {
                  challenge: b64u(assertion.publicKey.challenge),
                  rpId: assertion.publicKey.rpId,
                  allowCredentials: (assertion.publicKey.allowCredentials || []).map(
                    (c: any) => ({ ...c, id: b64u(c.id) })
                  ),
                  timeout: assertion.publicKey.timeout,
                  userVerification: assertion.publicKey.userVerification,
                };

                const assertCred = (await win2.navigator.credentials.get({
                  publicKey: getOpts,
                })) as PublicKeyCredential;
                expect(assertCred).to.not.be.null;

                const assertResp = assertCred.response as AuthenticatorAssertionResponse;
                const credJSON = {
                  id: assertCred.id,
                  rawId: toB64url(assertCred.rawId),
                  type: assertCred.type,
                  response: {
                    authenticatorData: toB64url(assertResp.authenticatorData),
                    clientDataJSON: toB64url(assertResp.clientDataJSON),
                    signature: toB64url(assertResp.signature),
                    userHandle: assertResp.userHandle ? toB64url(assertResp.userHandle) : null,
                  },
                };

                // Step 7: signin-flow に credential を送ってログイン完了
                // ここが FinishLogin success path の検証ポイント
                cy.request('POST', '/api/signin-flow', {
                  username: user.username,
                  password: user.password,
                  credential: credJSON,
                  sessionId: loginSessionId,
                }).then((loginRes) => {
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
