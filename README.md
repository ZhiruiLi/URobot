# transup: Compile and Copy Andorid Plugin to Unity Project

## Quick Start

Install with `go` command:

```bash
go get -u github.com/zhiruili/transup@latest
```

Compile Android module as AAR and extract it to Unity project:

```bash
transup -m mymodule -a ./AndroidProject -u ./UnityProject -e com.example.mymodule.MainActivity -B
```

Show help with `--help` argument:

```bash
transup --help
```
