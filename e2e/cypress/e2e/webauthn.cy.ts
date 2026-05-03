/// <reference types="cypress" />

// WebAuthn E2E: Chrome DevTools Protocol の Virtual Authenticator を使い、
// FIDO2 セキュリティキーの登録 → ログインのフルフローを検証する。
// Go 側の FinishLogin success path が実際のブラウザ WebAuthn API 経由で
// 動作することを確認する (#55, #698, #705)。

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

  it('registers a security key and logs in with it (TS-compat protocol)', () => {
    enableVirtualAuthenticator();

    // Step 1: ログインしてトークンを取得
    cy.request('POST', '/api/signin-flow', {
      username: user.username,
      password: user.password,
    }).then((signinRes) => {
      expect(signinRes.body.finished).to.be.true;
      const token = signinRes.body.i;
      const authHeader = { Authorization: `Bearer ${token}` };

      // 2FA 必須の前提条件: TOTP を有効化してから security key を登録する。
      // (本家 Misskey と同じく key 登録は 2FA enabled が前提)
      cy.request({
        method: 'POST',
        url: '/api/i/2fa/register',
        headers: authHeader,
        body: { password: user.password },
      }).then((reg2faRes) => {
        const totpSecret = reg2faRes.body.secret;
        expect(totpSecret).to.be.a('string').and.not.be.empty;

        cy.task<string>('totpCode', totpSecret).then((totpToken) => {
          cy.request({
            method: 'POST',
            url: '/api/i/2fa/done',
            headers: authHeader,
            body: { token: totpToken },
          }).then((doneRes) => {
            expect(doneRes.status).to.eq(200);

            // Step 2: register-key — 直接 PublicKeyCredentialCreationOptions
            // (no `publicKey` wrapper, no sessionId) が返る (#698)
            cy.task<string>('totpCode', totpSecret).then((regToken) => {
              cy.request({
                method: 'POST',
                url: '/api/i/2fa/register-key',
                headers: authHeader,
                body: { password: user.password, token: regToken },
              }).then((regKeyRes) => {
                expect(regKeyRes.status).to.eq(200);
                const creationOpts = regKeyRes.body;
                expect(creationOpts).to.have.property('rp');
                expect(creationOpts).to.have.property('challenge');
                // sessionId が無いことを確認 (user-keyed challenge)
                expect(regKeyRes.body).to.not.have.property('sessionId');

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

                  // Step 4: key-done に attestation を送信して登録完了。
                  // upstream 互換 body shape: { password, token, name, credential }
                  cy.task<string>('totpCode', totpSecret).then((doneToken) => {
                    cy.request({
                      method: 'POST',
                      url: '/api/i/2fa/key-done',
                      headers: authHeader,
                      body: {
                        password: user.password,
                        token: doneToken,
                        name: 'virtual-key',
                        credential: {
                          id: credential.id,
                          rawId: toB64url(credential.rawId),
                          type: credential.type,
                          response: {
                            clientDataJSON: toB64url(attResp.clientDataJSON),
                            attestationObject: toB64url(attResp.attestationObject),
                          },
                        },
                      },
                    }).then((kdRes) => {
                      expect(kdRes.status).to.eq(200);

                      // Step 5: signin-flow で password → 2FA + key step
                      cy.request('POST', '/api/signin-flow', {
                        username: user.username,
                        password: user.password,
                      }).then((step2Res) => {
                        // TS upstream 互換のレスポンス (#705):
                        //   next='passkey', authRequest=PublicKeyCredentialRequestOptions
                        //   sessionId / assertion フィールドは無い
                        expect(step2Res.body.finished).to.be.false;
                        expect(step2Res.body.next).to.eq('passkey');
                        expect(step2Res.body).to.have.property('authRequest');
                        expect(step2Res.body).to.not.have.property('sessionId');
                        expect(step2Res.body).to.not.have.property('assertion');

                        const authRequest = step2Res.body.authRequest;
                        // publicKey ラッパーが付いていないこと
                        expect(authRequest).to.not.have.property('publicKey');
                        expect(authRequest.challenge).to.be.a('string');

                        // Step 6: ブラウザの WebAuthn API で assertion を取得
                        cy.window().then(async (win2) => {
                          const getOpts: PublicKeyCredentialRequestOptions = {
                            challenge: b64u(authRequest.challenge),
                            rpId: authRequest.rpId,
                            allowCredentials: (authRequest.allowCredentials || []).map(
                              (c: any) => ({ ...c, id: b64u(c.id) })
                            ),
                            timeout: authRequest.timeout,
                            userVerification: authRequest.userVerification,
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

                          // Step 7: signin-flow に credential を送ってログイン完了。
                          // sessionId は送らない (server-side が user-keyed で保持)。
                          cy.request('POST', '/api/signin-flow', {
                            username: user.username,
                            password: user.password,
                            credential: credJSON,
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
        });
      });
    });
  });
});
