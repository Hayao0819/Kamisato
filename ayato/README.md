# Ayato | 綾人

Ayato is a Blinkyd compatible backend for ayaka and blinky.It handles package hosting
with automatic database updates.

## Features

- Upload package file with `blinky` or `ayaka` command
- Delete package file with `blinky` command
- Auto update repository database
- バイナリをS3かローカルに保存
- メタデータをSQLかローカルに保存
- 最小限のArchLinux依存

## Todo

- Implement basic features
  - Store metadata
  - Provide stored data
- Multi-arch support
- GPG sign support
- API for lumine
