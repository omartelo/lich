// A GUI lifecycle with a child holding a bundled DLL, without downloading CEF.
using System;
using System.Diagnostics;
using System.IO;
using System.Threading;
using System.Windows.Forms;

class WindowFixture {
    static void Record(string message) {
        File.AppendAllText(Environment.GetEnvironmentVariable("LICH_UPDATE_EVENTS"), message + "\n");
    }

    [STAThread]
    static void Main(string[] args) {
        string resource = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "libcef.dll");
        if (args.Length == 1 && args[0] == "hold") {
            using (FileStream held = File.Open(resource, FileMode.Open, FileAccess.Read, FileShare.Read)) {
                Console.WriteLine("locked");
                Console.ReadLine();
                Thread.Sleep(1000); // Children take time to release their loaded resources.
            }
            Record("locks released");
            return;
        }
        string version = File.ReadAllText(resource).Trim();
        using (Process child = Process.Start(new ProcessStartInfo(Application.ExecutablePath, "hold") {
            UseShellExecute = false, RedirectStandardInput = true, RedirectStandardOutput = true
        })) {
            if (child.StandardOutput.ReadLine() != "locked") throw new Exception("child did not lock DLL");
            using (Form window = new Form()) {
                window.Text = "lich update fixture " + version;
                window.Shown += delegate { Record("window ready " + version); };
                window.FormClosing += delegate {
                    Record("close requested " + version);
                    child.StandardInput.WriteLine("close");
                    child.StandardInput.Flush();
                    if (!child.WaitForExit(10000)) throw new Exception("child retained DLL");
                    Record("window closed " + version);
                };
                Application.Run(window);
            }
        }
    }
}
