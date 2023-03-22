## Note
As this library is stored in a private repository, in order to use it in your code, you might need to:

### 1. Configurig git to using SSH instead of HTTPS
Run the below command (only need to run once):

```bash
git config --global url."ssh://git@git.rnd.amadeus.net/".insteadOf "https://rndwww.nce.amadeus.net/git/scm/"
```

### 2.: Using `GOPRIVATE`
Run

```bash
go env -w GOPRIVATE='rndwww.nce.amadeus.net/*'
```

if you have multiple private modules, just put them in a list separated by comma (`,`).

or 
```bash
export GOPRIVATE=*
```