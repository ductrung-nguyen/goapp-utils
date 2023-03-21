## Note
As this library is stored in a private repository, in order to use it in your code, you might need to:

### Method 1:  using GIT instead of HTTPS.
Run the below command (only need to run once):

```bash
git config --global url."ssh://git@git.rnd.amadeus.net/splunk/goapp-utils.git".insteadOf "https://rndwww.nce.amadeus.net/git/projects/SPLUNK/repos/goapp-utils/"
```

### Method 2: Using `replace` in `go.mod`
or putting this into your `go.mod`

```text
replace (
    # use "go get -insecure gitlab.my-company.com/my-team/my-library" to get latest commit hash
    rndwww.nce.amadeus.net/git/projects/SPLUNK/repos/goapp-utils => ssh://git@git.rnd.amadeus.net/splunk/goapp-utils.git
)
```

### Method 3: Using `GOPRIVATE`
Run

```bash
go env -w GOPRIVATE=rndwww.nce.amadeus.net/git/projects/SPLUNK/repos/*
```

if you have multiple private modules, just put them in a list separated by comma (`,`).

or 
```bash
export GOPRIVATE=*
```