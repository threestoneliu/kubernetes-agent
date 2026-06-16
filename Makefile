.PHONY: web copy-web build run test vet clean

# web builds the SPA. --frozen-lockfile makes pnpm fail if
# pnpm-lock.yaml would need to change, so the lockfile is the
# single source of truth for web deps.
web:
	cd web && pnpm install --frozen-lockfile && pnpm build

# copy-web stages web/dist/ into the package the embed.FS lives
# in. go:embed cannot reference paths outside its own directory,
# so we mirror the build output here. The destination is a build
# artifact and is gitignored.
copy-web: web
	rm -rf internal/server/web_dist
	mkdir -p internal/server/web_dist
	cp -R web/dist/. internal/server/web_dist/

# build runs web + copy + go build. GOSUMDB=sum.golang.org is
# prefixed so callers don't need to remember the env var.
build: copy-web
	GOSUMDB=sum.golang.org go build -o kubernetes-agent ./cmd/server

run: build
	./kubernetes-agent

test:
	GOSUMDB=sum.golang.org go test -count=1 ./...

vet:
	GOSUMDB=sum.golang.org go vet ./...

clean:
	rm -rf internal/server/web_dist kubernetes-agent web/dist