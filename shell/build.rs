// Windows reads an executable's icon and its manifest out of the executable
// itself: Explorer, the Start Menu shortcut and the taskbar draw the icon
// resource, and Chromium's own startup checks the manifest's compatibility
// section to learn which Windows it is on. Both are compiled in here. On the
// other platforms this script does nothing.
fn main() {
    #[cfg(windows)]
    {
        let icon = "../build/windows/lich.ico";
        let manifest = "../build/windows/shell/lich-shell.exe.manifest";
        println!("cargo:rerun-if-changed={icon}");
        println!("cargo:rerun-if-changed={manifest}");
        winresource::WindowsResource::new()
            .set_icon(icon)
            .set_manifest_file(manifest)
            .set("ProductName", "lich")
            .set("FileDescription", "lich")
            .compile()
            .expect("compile the window's icon and manifest into the executable");
    }
}
