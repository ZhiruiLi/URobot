package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
)

func unzip(srcFile, dstDir string) error {
	archive, err := zip.OpenReader(srcFile)
	if err != nil {
		panic(err)
	}
	defer archive.Close()

	for _, f := range archive.File {
		filePath := filepath.Join(dstDir, f.Name)

		if !strings.HasPrefix(filePath, filepath.Clean(dstDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path")
		}

		if f.FileInfo().IsDir() {
			logTrace("creating directory %s ...", filePath)
			os.MkdirAll(filePath, os.ModePerm)
			continue
		}

		logTrace("unzipping file %s ...", filePath)

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		fileInArchive, err := f.Open()
		if err != nil {
			return err
		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return err
		}

		dstFile.Close()
		fileInArchive.Close()
	}
	return nil
}

type options struct {
	// Slice of bool will append 'true' each time the option is encountered (can be set multiple times, like -vvv)
	Verbose            []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	ModuleName         string `short:"m" long:"module-name" description:"Android module name" required:"true"`
	AndroidProjectPath string `short:"a" long:"android-path" description:"Android project path" required:"true"`
	UnityProjectPath   string `short:"u" long:"unity-path" description:"Unity project path" required:"true"`
	EntryActivity      string `short:"e" long:"entry-activity" description:"Full name of entry activity " required:"true"`
}

var opts options

func (o *options) pluginBaseDir() string {
	return filepath.Join(o.UnityProjectPath, "Plugins", "Android")
}

func (o *options) currentPluginDir() string {
	return filepath.Join(o.pluginBaseDir(), opts.ModuleName)
}

func (o *options) moduleDir() string {
	return filepath.Join(o.AndroidProjectPath, o.ModuleName)
}

func (o *options) moduleAarDir() string {
	return filepath.Join(o.moduleDir(), "build", "outputs", "aar")
}

func (o *options) moduleAarFile() string {
	return filepath.Join(o.moduleAarDir(), fmt.Sprintf("%s-%s.aar", o.ModuleName, "debug"))
}

func (o *options) isDebug() bool {
	return len(o.Verbose) >= 1
}

func (o *options) isVerbose() bool {
	return len(o.Verbose) >= 2
}

func errorf(f string, a ...interface{}) {
	fmt.Printf(f, a...)
}

func debugf(f string, a ...interface{}) {
	if opts.isDebug() {
		fmt.Printf(f, a...)
	}
}

func tracef(f string, a ...interface{}) {
	if opts.isVerbose() {
		fmt.Printf(f, a...)
	}
}

func logTrace(f string, a ...interface{}) {
	tracef(f+"\n", a...)
}

func logDebug(f string, a ...interface{}) {
	debugf(f+"\n", a...)
}

func logError(f string, a ...interface{}) {
	errorf(f+"\n", a...)
}

type funcWriter func(f string, a ...interface{})

func (f funcWriter) Write(data []byte) (n int, err error) {
	f("%s", string(data))
	return len(data), nil
}

func setAbsPath(tag string, path *string) error {
	newPath, err := filepath.Abs(*path)
	if err != nil {
		return fmt.Errorf("illegal %s path %s: %w", tag, *path, err)
	}
	*path = newPath
	return nil
}

func chdir(path string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if err := os.Chdir(path); err != nil {
		return "", err
	}
	return cwd, nil
}

func checkFileExist(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		return fmt.Errorf("not a file %s", path)
	}
	return nil
}

func checkDirExist(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("not a directory %s", path)
	}
	return nil
}

func runCommandAt(path string, cmdName string, args ...string) error {
	if cwd, err := chdir(path); err != nil {
		return err
	} else {
		defer chdir(cwd)
	}
	cmd := exec.Command(cmdName, args...)
	cmd.Stdout = funcWriter(debugf)
	cmd.Stderr = funcWriter(errorf)
	return cmd.Run()
}

func buildAndroid(path string) error {
	if err := runCommandAt(path, "gradlew.bat", "assembleDebug"); err != nil {
		return fmt.Errorf("build Android project fail %w", err)
	}
	return nil
}

func makeDir(path string, deleteOrigin bool) error {
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.MkdirAll(path, os.ModePerm)
		}
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s existed and not a directory", path)
	}
	if !deleteOrigin {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("fail to delete origin directory at %s", path)
	}
	return os.Mkdir(path, os.ModePerm)
}

func addPropertiesFile(dir string) error {
	path := filepath.Join(dir, "project.properties")
	return ioutil.WriteFile(path, []byte("android.library=true"), 0644)
}

const manifestTemplate string = `<?xml version="1.0" encoding="utf-8"?>
<manifest
    xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.unity3d.player"
    android:installLocation="preferExternal"
    android:versionCode="1"
    android:versionName="1.0">
    <supports-screens
        android:smallScreens="true"
        android:normalScreens="true"
        android:largeScreens="true"
        android:xlargeScreens="true"
        android:anyDensity="true"/>

    <application
        android:theme="@style/UnityThemeSelector"
        android:icon="@drawable/app_icon"
        android:label="@string/app_name"
        android:debuggable="true">
        <activity android:name="${ACTIVITY_NAME}"
                  android:label="@string/app_name">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
            <meta-data android:name="unityplayer.UnityActivity" android:value="true" />
        </activity>
    </application>
</manifest>`

func addAndroidManifestFile(dir string, entryActivity string) error {
	var xml = strings.ReplaceAll(manifestTemplate, "${ACTIVITY_NAME}", entryActivity)
	path := filepath.Join(dir, "AndroidManifest.xml")
	return ioutil.WriteFile(path, []byte(xml), 0644)
}

func main1() error {
	if err := setAbsPath("Android project", &opts.AndroidProjectPath); err != nil {
		return err
	}

	if err := setAbsPath("Unity project", &opts.UnityProjectPath); err != nil {
		return err
	}

	if err := checkDirExist(opts.AndroidProjectPath); err != nil {
		return fmt.Errorf("Android project no found: %w", err)
	}
	logTrace("Android project at: %s", opts.AndroidProjectPath)

	if err := checkDirExist(opts.moduleDir()); err != nil {
		return fmt.Errorf("module %s no found: %w", opts.ModuleName, err)
	}
	logTrace("Module %s project at: %s", opts.ModuleName, opts.moduleDir())

	if err := checkDirExist(opts.UnityProjectPath); err != nil {
		return fmt.Errorf("Unity project no found: %w", err)
	}
	logTrace("Unity project at: %s", opts.UnityProjectPath)

	logTrace("start building Android project ...")
	if err := buildAndroid(opts.AndroidProjectPath); err != nil {
		return err
	}

	if err := checkFileExist(opts.moduleAarFile()); err != nil {
		return fmt.Errorf("Android build result no found: %w", err)
	}

	if err := makeDir(opts.pluginBaseDir(), false); err != nil {
		return err
	}
	logTrace("Android plugin base directory at: %s", opts.pluginBaseDir())

	if err := makeDir(opts.currentPluginDir(), true); err != nil {
		return err
	}
	logTrace("Android current plugin directory at: %s", opts.currentPluginDir())

	logTrace("start unzipping aar ...")
	if err := unzip(opts.moduleAarFile(), opts.currentPluginDir()); err != nil {
		return err
	}

	logTrace("start generating properties file ...")
	if err := addPropertiesFile(opts.currentPluginDir()); err != nil {
		return err
	}

	logTrace("start generating Android manifest file ...")
	if err := addAndroidManifestFile(opts.pluginBaseDir(), opts.EntryActivity); err != nil {
		return err
	}

	return nil
}

func main() {
	if _, err := flags.ParseArgs(&opts, os.Args); err != nil {
		panic(err)
	}

	if err := main1(); err != nil {
		logError(err.Error())
	}
}
