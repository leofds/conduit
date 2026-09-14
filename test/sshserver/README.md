# Test SSH server

A real OpenSSH `sshd` in a container, used to run conduit's SSH integration tests
against a genuine SSH server (PTY, resize, `Setenv`, host-key TOFU, keepalive, auth).

## Users

| User     | Password     | Rights                                              |
|----------|--------------|-----------------------------------------------------|
| `master` | `masterpass` | Passwordless `sudo` (NOPASSWD) — can run as root    |
| `guest`  | `guestpass`  | Limited account; explicitly denied all `sudo`       |

Both users accept password auth and key-based auth. The keypair is generated on
the host by `make sshserver-keys` (into `test/sshserver/keys/`, which is
git-ignored). The public key is mounted into the container and installed as
`authorized_keys` for both users; the private key stays on the host so tests and
manual `ssh` clients can authenticate with it.

## Generate the test keypair

```bash
make sshserver-keys
# -> test/sshserver/keys/authorized_keys.test       (private, keep on host)
# -> test/sshserver/keys/authorized_keys.test.pub   (public, mounted into the container)
```

`make sshserver-up` runs this automatically, so the key is always present before
the container starts.

## Run

Using make (recommended):

```bash
make sshserver-up          # generates keys if missing, builds + starts on 127.0.0.1:2222
make sshserver-shell       # master shell inside the container
make sshserver-down
```

Using docker compose directly:

```bash
docker compose -f test/sshserver/docker-compose.test.yml up -d --build
docker compose -f test/sshserver/docker-compose.test.yml down
```

The server listens on the host at `127.0.0.1:2222`.

## Hosts config for conduit

```yaml
# hosts.yaml
test-master:
  address: 127.0.0.1
  port: "2222"
  username: master
  password: masterpass
  auto_accept_host_key: true   # avoid the interactive TOFU prompt in automation

test-key-master:
  address: 127.0.0.1
  port: "2222"
  username: master
  private_key_file: ../test/sshserver/keys/authorized_keys.test
  auto_accept_host_key: true
```

## Example checks

```bash
# Password auth — as master: sudo works (runs as root)
ssh -p 2222 master@127.0.0.1          # password: masterpass
#   $ sudo id    -> uid=0(root) ...

# Password auth — as guest: sudo is denied
ssh -p 2222 guest@127.0.0.1           # password: guestpass
#   $ sudo id    -> "guest is not allowed to execute ... as root"

# Key auth — using the host-generated key (no password prompt)
ssh -p 2222 -i test/sshserver/keys/authorized_keys.test master@127.0.0.1
```

## Notes

- Host keys are generated at image build time, so the fingerprint is stable for a
  given image. After a rebuild you may need `conduit -R test-master` or keep
  `auto_accept_host_key: true`.
- `AcceptEnv CONDUIT_* TEST_*` is set in `sshd_config`, so conduit's `Setenv`
  calls are accepted — useful for testing the env-forwarding path.
