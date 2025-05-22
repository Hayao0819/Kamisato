# ベースイメージの指定（軽量なalpineベースを使用）
FROM golang:1.24-alpine

# 必要なツールのインストール（例: git）
RUN apk add --no-cache git

# 作業ディレクトリ作成
WORKDIR /app

# Goモジュールのキャッシュ活用のためgo.modとgo.sumを先にコピー
COPY go.mod go.sum ./
RUN go mod download

# プロジェクト全体をコピー
COPY alpm ./alpm
COPY ayaka ./ayaka
COPY ayato ./ayato
COPY conf ./conf
COPY example ./example
COPY internal ./internal
COPY raiou ./raiou
COPY run_ayaka.sh .
COPY run_ayato.sh .
COPY run_lumine.sh .
COPY LICENSE.txt .
COPY README.md .
COPY main.go .

# ayato/main.go をビルド（出力バイナリは ayato）
RUN go build -o ayato ./ayato/main.go

# ポート番号（必要に応じて）
EXPOSE 8080

# 起動コマンド（ビルドしたバイナリを実行）
CMD ["./ayato"]
