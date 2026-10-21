# Smol Auto Clicker

<img src="assets/icon.png" alt="Smol Auto Clicker icon" width="128" height="128">

A minimal Windows autoclicker with repeat-click and button-hold modes. Written in
Go using Win32 directly, with no third-party dependencies or runtime to install.

## Use

1. Choose **Left**, **Right**, or **Middle**, then **Click repeatedly** or **Hold down**.
2. Set the click interval (default: 100 ms) and start delay (default: 0 ms).
3. Point at your target. Press **F6** to start or stop; **F7** always stops.

Both modes follow your cursor, and shortcuts work while other apps have focus.
Hold mode keeps the button pressed until you stop. If a release is blocked,
return to the desktop and press F7 again.

Windows may block input to elevated apps, and some apps reject simulated input.

## Build

Requires Go 1.27.3 or later. On Windows:

```sh
go build -trimpath -ldflags="-H=windowsgui -s -w" -o smol-auto-clicker.exe .
```

Run `smol-auto-clicker.exe`; no other files are needed. To cross-compile from
Linux or macOS, prefix the build command with
`GOOS=windows GOARCH=amd64 CGO_ENABLED=0`. Use `GOARCH=arm64` for Windows on ARM.

<details>
<summary>Updating the icon or Windows file details</summary>

The checked-in `.syso` files embed the icon and file details. After changing
`assets/icon.ico` or `assets/resources.rc`, regenerate both files with LLVM's
resource tools:

```sh
cd assets
llvm-rc /no-preprocess /fo resources.res resources.rc
llvm-cvtres /machine:x64 /timestamp:0 /out:../resources_windows_amd64.syso resources.res
llvm-cvtres /machine:arm64 /timestamp:0 /out:../resources_windows_arm64.syso resources.res
```

</details>

## License

[MIT](LICENSE)
