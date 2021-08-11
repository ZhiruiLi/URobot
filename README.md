# upack: Transport Android Plugin for Unity

## Quick Start

Install with `go` command:

```bash
go get -u github.com/zhiruili/upack@latest
```

Compile Android module as AAR and extract it to Unity project:

```bash
upack -m mymodule -a ./AndroidProject -u ./UnityProject -e com.example.mymodule.MainActivity -B
```

Show help with `--help` argument:

```bash
upack --help
```
