_default:
  @just --choose

test:
  go test ./...

# "statusLine": {
#   "type": "command",
#   "command": "bash \"$HOME/.claude/statusline.sh\""
# }
build:
  CGO_ENABLED=0 go build -o bin/skein .

install:
  ./bin/skein install
