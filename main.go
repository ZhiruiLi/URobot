package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jessevdk/go-flags"
)

type options struct {
	// Slice of bool will append 'true' each time the option is encountered (can be set multiple times, like -vvv)
	Verbose                 []bool   `short:"v" long:"verbose" description:"Show verbose debug information"`
	AndroidModuleName       string   `short:"m" long:"android-module-name" env:"UPACK_ANDROID_MODULE_NAME" description:"Android module name" required:"true"`
	AndroidProjectPath      string   `short:"a" long:"android-path" env:"UPACK_ANDROID_PROJECT_PATH" description:"Android project path" required:"true"`
	UnityProjectPath        string   `short:"u" long:"unity-path" env:"UPACK_UNITY_PROJECT_PATH" description:"Unity project path" required:"true"`
	AndroidEntryActivity    string   `short:"e" long:"entry-activity" env:"UPACK_ENTRY_ACTIVITY" description:"Full name of entry activity " required:"true"`
	AndroidPermissions      []string `short:"p" long:"android-permissions" env:"UPACK_ANDROID_PERMISSIONS" description:"Acquire permissions in Android manifest" required:"false"`
	AndroidManifestTemplate string   `short:"T" long:"manifest-template" env:"UPACK_MANIFEST_TEMPLATE" description:"Android manifest template file path" required:"false"`
	BackupExtension         string   `short:"B" long:"backup-extension" env:"UPACK_BACKUP_EXTENSION" description:"Keep the original files with the given ext name" required:"false"`
}

var opts options

func (o *options) pluginBaseDir() string {
	return filepath.Join(o.UnityProjectPath, "Assets", "Plugins", "Android")
}

func (o *options) currentPluginDir() string {
	return filepath.Join(o.pluginBaseDir(), opts.AndroidModuleName)
}

func (o *options) moduleDir() string {
	return filepath.Join(o.AndroidProjectPath, o.AndroidModuleName)
}

func (o *options) moduleAarDir() string {
	return filepath.Join(o.moduleDir(), "build", "outputs", "aar")
}

func (o *options) moduleAarFile() string {
	return filepath.Join(o.moduleAarDir(), fmt.Sprintf("%s-%s.aar", o.AndroidModuleName, "debug"))
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
	if err := runCommandAt(path, "gradlew", "assembleDebug"); err != nil {
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

func renameIfExist(path, newPath string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	if err := os.RemoveAll(newPath); err != nil {
		return err
	}

	return os.Rename(path, newPath)
}

func backupAndWriteFile(path string, content []byte, backupExt string) error {
	if err := removeOrBackup(path, backupExt); err != nil {
		return err
	}
	return ioutil.WriteFile(path, content, 0644)
}

func addPropertiesFile(dir string, backupExt string) error {
	path := filepath.Join(dir, "project.properties")
	return backupAndWriteFile(path, []byte("android.library=true"), backupExt)
}

const defaultManifestTemplate string = `<?xml version="1.0" encoding="utf-8"?>
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
{{range .AndroidPermissions}}
    <uses-permission android:name="{{.}}" />
{{- end}}

    <application
        android:theme="@style/UnityThemeSelector"
        android:icon="@drawable/app_icon"
        android:label="@string/app_name"
        android:debuggable="true">
        <activity android:name="{{.AndroidEntryActivity}}"
                  android:label="@string/app_name">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
            <meta-data android:name="unityplayer.UnityActivity" android:value="true" />
        </activity>
    </application>
</manifest>`

func loadManifestTemplateContent(path string) (string, error) {
	if path == "" {
		return defaultManifestTemplate, nil
	}
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func loadManifestTemplate(path string) (*template.Template, error) {
	content, err := loadManifestTemplateContent(path)
	if err != nil {
		return nil, err
	}
	name := "DefaultManifest"
	if path != "" {
		name = "Manifest:" + path
	}
	return template.New(name).Parse(content)
}

func addAndroidManifestFile(dir string, content []byte, backupExt string) error {
	path := filepath.Join(dir, "AndroidManifest.xml")
	return backupAndWriteFile(path, content, backupExt)
}

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

func removeOrBackup(path string, backupExt string) error {
	if len(backupExt) > 0 {
		bpath := path + backupExt
		if err := renameIfExist(path, bpath); err != nil {
			return fmt.Errorf("backup %s: %w", path, err)
		}
	} else {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("delete %s: %w", path, err)
		}
	}
	return nil
}

func cleanAndUnzipFile(srcFile, dstDir string, backupExt string) error {
	if err := removeOrBackup(dstDir, backupExt); err != nil {
		return err
	}
	return unzip(srcFile, dstDir)
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
		return fmt.Errorf("module %s no found: %w", opts.AndroidModuleName, err)
	}
	logTrace("Module %s project at: %s", opts.AndroidModuleName, opts.moduleDir())

	if err := checkDirExist(opts.UnityProjectPath); err != nil {
		return fmt.Errorf("Unity project no found: %w", err)
	}
	logTrace("Unity project at: %s", opts.UnityProjectPath)

	tmpl, err := loadManifestTemplate(opts.AndroidManifestTemplate)
	if err != nil {
		return fmt.Errorf("Android manifest template load fail: %w", err)
	}
	var manifestBuf bytes.Buffer
	if err := tmpl.Execute(&manifestBuf, opts); err != nil {
		return fmt.Errorf("Andoird manifest generate fail: %w", err)
	}

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
	if err := cleanAndUnzipFile(opts.moduleAarFile(), opts.currentPluginDir(), opts.BackupExtension); err != nil {
		return err
	}

	logTrace("start generating properties file ...")
	if err := addPropertiesFile(opts.currentPluginDir(), opts.BackupExtension); err != nil {
		return err
	}

	logTrace("start generating Android manifest file ...")
	if err := addAndroidManifestFile(opts.pluginBaseDir(), manifestBuf.Bytes(), opts.BackupExtension); err != nil {
		return err
	}

	return nil
}

func main() {
	if _, err := flags.ParseArgs(&opts, os.Args); err != nil {
		return
	}

	if err := main1(); err != nil {
		logError(err.Error())
		return
	}
}
