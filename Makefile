
bin:
	goreleaser --snapshot --skip-publish --rm-dist

release:
	goreleaser release --rm-dist

	# update the pkg.go.dev cache
	GOPROXY=https://proxy.golang.org GO111MODULE=on go get github.com/$(USER)/$(PACKAGE)@v$(VERSION)