
.PHONY: compile


compile:  # Compile the code
	go build -v -trimpath -ldflags "-s -w" \
    -o  "binary" . \
    || return 1

# -X $pkg.gitVersion=$(git describe --tags --abbrev=0 || echo unknown)" \
# -X $pkg.gitCommit=$(git rev-parse HEAD 2>/dev/null || echo unknown) \
# -X $pkg.gitTreeState=$git_tree_state \
# -X $pkg.buildDate=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
    