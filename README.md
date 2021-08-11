# upack: Transport Android Plugin for Unity

## Quick Start

Install with `go` command:

```bash
go get -u github.com/zhiruili/upack@latest
```

Compile Android module as AAR and extract it to Unity project:

```bash
# build Android module under ./AndroidProject and extra AAR to ./UnityProject/Assets/Plugins/Android
upack -m mymodule -a ./AndroidProject -e com.example.mymodule.MainActivity ./UnityProject/Assets/Plugins/Android
```

Show help with `--help` argument:

```bash
upack --help
```
