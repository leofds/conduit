# Test SSH server for conduit integration tests.
#
# Runs a real OpenSSH sshd with two users:
#   - master  : can run commands as root via passwordless sudo (NOPASSWD)
#   - guest   : limited account, no sudo, regular permissions
#
# Build:
#   docker build -t conduit-sshtest test/sshserver
#
# Run (host port 2222 -> container 22):
#   docker run -d --rm --name conduit-sshtest -p 2222:22 conduit-sshtest
#
FROM alpine:3.20

RUN apk add --no-cache openssh sudo bash shadow netcat-openbsd

# Generate a stable host key set at build time so the TOFU fingerprint is
# reproducible across rebuilds of the same image.
RUN ssh-keygen -A

# Show the username in the shell prompt. Alpine's stock /etc/profile sets
# PS1='\h:\w\$ ' (no \u), so master and guest look identical. /etc/profile
# sources /etc/profile.d/*, so this applies to all login shells.
RUN printf '%s\n' "export PS1='\\u@\\h:\\w\\$ '" > /etc/profile.d/prompt.sh

COPY sshd_config /etc/ssh/sshd_config
COPY banner.txt /etc/ssh/banner.txt
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 22

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["/usr/sbin/sshd", "-D", "-e"]
