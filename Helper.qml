import QtQuick
import Quickshell.Io

// One short-lived request to the mwbcfg helper. Arguments are an argv array,
// never a shell string. Secrets go in `stdinText` (written once, then
// dropped), never in argv. Emits `finished(result)` with the parsed JSON, or
// {ok:false, error} if the helper could not run or timed out.
Item {
  id: root

  property string stdinText: ""
  property bool done: false
  signal finished(var result)

  function start(argv, input) {
    stdinText = input || ""
    proc.stdinEnabled = stdinText !== ""
    proc.command = argv
    watchdog.start()
    proc.running = true
  }

  function finish(result) {
    if (done) return
    done = true
    watchdog.stop()
    stdinText = ""
    finished(result)
    root.destroy()
  }

  Process {
    id: proc
    stdout: StdioCollector { id: out }
    onStarted: {
      if (root.stdinText !== "") {
        proc.write(root.stdinText + "\n")
        root.stdinText = ""
      }
    }
    onExited: function(code) {
      var parsed = null
      try { parsed = JSON.parse(String(out.text).trim().split("\n").pop()) } catch (e) {}
      root.finish(parsed && typeof parsed === "object" ? parsed : { ok: false, error: "helper" })
    }
  }

  Timer {
    id: watchdog
    interval: 25000
    onTriggered: { proc.running = false; root.finish({ ok: false, error: "timeout" }) }
  }
}
