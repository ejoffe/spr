VERSION = 0.7.3

bin:
	goreleaser --snapshot --skip-publish --rm-dist

release:
	goreleaser release --rm-dist

	# update the pkg.go.dev cache
	GOPROXY=https://proxy.golang.org GO111MODULE=on go get github.com/ejoffe/spr@v$(VERSION)
