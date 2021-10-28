
bin:
	goreleaser --snapshot --skip-publish --rm-dist

release:
	goreleaser release --rm-dist

