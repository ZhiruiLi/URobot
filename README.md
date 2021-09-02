# upack: 将 Android 工程集成到 Unity 中的辅助工具

## 快速开始

使用 `go` 命令安装：

```bash
go install github.com/zhiruili/upack@latest
```

编译 Android 模块并拷贝到 Unity 工程中：

```bash
# build Android module under ./AndroidProject and extra AAR to ./UnityProject/Assets/Plugins/Android
upack -m mymodule -a ./AndroidProject -e com.example.mymodule.MainActivity ./UnityProject/Assets/Plugins/Android
```

通过 `--help` 参数来显示帮助信息：

```bash
upack --help
```

## 示例工程

参考：[UnityAndroidExample](https://github.com/ZhiruiLi/UnityAndroidExample)。