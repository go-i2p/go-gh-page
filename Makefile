<<<<<<< HEAD
fmt: cp
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;

cp:
	cp -v ./.github/workflows/page.yml pkg/templates/page.yml
||||||| 883cc95
fmt:
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;
=======
fmt:
	find . -name '*.go' -exec gofumpt -w -s -extra {} \;
>>>>>>> a0804fdff6ce1cf2312908d85889ae632c2915e6
