import QtQuick
import QtQuick.Controls
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Commons as Commons
import qs.Ui

// Mouse Without Borders: one bar icon, one panel. All config and service work
// is done by the mwbcfg helper (see helper/). The security key is only ever
// held in the key field while the panel is open and is sent to the helper on
// stdin, never in argv.
Panel {
  id: root
  moduleName: "sippin45.mwb"
  ipcTarget: "sippin45.mwb"

  // The bar sizes the widget from these; without them it collapses to zero.
  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  readonly property string pluginDir: String(Qt.resolvedUrl(".")).replace(/^file:\/\//, "").replace(/\/+$/, "")
  readonly property string helperPath: pluginDir + "/bin/mwbcfg"

  readonly property color foreground: bar ? bar.foreground : Commons.Color.foreground
  readonly property color urgent: bar ? bar.urgent : Commons.Color.urgent
  readonly property color dim: Qt.darker(foreground, 1.55)
  readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

  Commons.Translations { id: strings; directory: Qt.resolvedUrl("i18n") }

  // ---- state ----
  property var cfg: ({ host: "", keySet: false, fileFetch: "immediate", clipboard: true, port: 0 })
  property bool cfgLoaded: false
  property string svcState: "unknown"
  property string conn: "unknown"
  property bool needsRestart: false
  property bool busy: false
  property string message: ""
  property bool messageIsError: false
  property bool helperMissing: false

  property string draftHost: ""
  property string draftKey: ""
  property string draftFileFetch: "immediate"
  property bool draftClipboard: true
  property bool showKey: false

  readonly property bool dirty: cfgLoaded && (draftHost !== cfg.host || draftKey !== ""
    || draftFileFetch !== cfg.fileFetch || draftClipboard !== cfg.clipboard)
  readonly property bool running: svcState === "active" || svcState === "activating"
  readonly property bool canToggle: svcState !== "unknown" && svcState !== "deactivating" && !busy

  function stateLabel() {
    var s = strings.t("state." + svcState)
    if (svcState === "active" && (conn === "connected" || conn === "connecting"))
      s += " · " + strings.t("conn." + conn)
    return s
  }

  function say(text, isError) {
    message = text
    messageIsError = !!isError
  }

  function run(argv, input, cb) {
    var h = helperComponent.createObject(root)
    h.finished.connect(cb)
    h.start(argv, input)
  }

  function loadConfig() {
    run([helperPath, "get"], "", function(r) {
      if (!r.ok) {
        helperMissing = r.error === "helper"
        say(helperMissing ? strings.t("helperMissing") : strings.t("loadFailed"), true)
        return
      }
      helperMissing = false
      cfg = r.config
      cfgLoaded = true
      draftHost = cfg.host
      draftFileFetch = cfg.fileFetch
      draftClipboard = cfg.clipboard
      draftKey = ""
    })
  }

  function refreshStatus() {
    run([helperPath, "status"], "", function(r) {
      if (!r.ok) return
      svcState = r.state
      conn = r.conn
      needsRestart = !!r.needsRestart
    })
  }

  function serviceAction(action) {
    if (busy) return
    busy = true
    say("", false)
    run([helperPath, "action", action], "", function(r) {
      busy = false
      if (!r.ok) say(r.error, true)
      refreshStatus()
    })
  }

  function save() {
    if (busy || !dirty) return
    busy = true
    say("", false)
    var change = { host: draftHost.trim(), file_fetch: draftFileFetch, clipboard: draftClipboard }
    if (draftKey !== "") change.key = draftKey
    run([helperPath, "set"], JSON.stringify(change), function(r) {
      busy = false
      if (!r.ok) { say(r.error, true); return }
      draftKey = ""
      showKey = false
      loadConfig()
      say(strings.t("saved"), false)
      refreshStatus()
    })
  }

  function testHost() {
    if (busy) return
    var host = draftHost.trim()
    busy = true
    say(strings.t("testing"), false)
    run([helperPath, "test-host", host], "", function(r) {
      busy = false
      if (!r.ok) say(r.error, true)
      else say(strings.t(r.reachable ? "reachable" : "unreachable").arg(host), !r.reachable)
    })
  }

  // The key must not outlive the open panel.
  onOpenedChanged: {
    if (opened) {
      say("", false)
      loadConfig()
      refreshStatus()
    } else {
      draftKey = ""
      showKey = false
    }
  }

  // Fast while the panel is open, slow (for the tooltip) while closed.
  Timer {
    interval: root.opened ? 4000 : 30000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: root.refreshStatus()
  }

  Component { id: helperComponent; Helper {} }

  BarIconButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    // The icon is design/mwb-icon.svg, tinted with the bar's foreground so it
    // matches the other icons and follows the theme. Edit the SVG, not this.
    iconComponent: Component {
      Item {
        Image {
          id: art
          anchors.fill: parent
          visible: false
          source: Qt.resolvedUrl("design/mwb-icon.svg")
          sourceSize: Qt.size(96, 96)
          fillMode: Image.PreserveAspectFit
          smooth: true
          mipmap: true
        }
        MultiEffect {
          anchors.fill: art
          source: art
          colorization: 1.0
          colorizationColor: root.barForeground
        }
      }
    }
    tooltipText: strings.t("tooltip").arg(root.stateLabel())
    onPressed: function(buttonCode) { root.toggle() }
  }

  KeyboardPanel {
    id: panel
    anchorItem: button
    owner: root
    bar: root.bar
    open: root.opened
    focusTarget: panel.loadedContent
    contentWidth: panel.fittedContentWidth(Style.space(360))
    contentHeight: panel.fittedContentHeight(panel.loadedContentHeight, Style.space(640))

    content: PanelKeyCatcher {
      id: keyCatcher
      implicitHeight: column.implicitHeight
      anchors.fill: parent
      // Text fields take their own keys; Escape still closes the panel.
      blocked: hostField.activeFocus || keyField.activeFocus
      onCloseRequested: root.close()

      Column {
        id: column
        width: parent.width
        spacing: Style.space(14)

        PanelHero {
          width: parent.width
          title: strings.t("title")
          foreground: root.foreground
          fontFamily: root.fontFamily
          trailingControl: Component {
            ToggleSwitch {
              checked: root.running
              busy: root.busy
              enabled: root.canToggle
              foreground: root.foreground
              onToggled: root.serviceAction(root.running ? "stop" : "start")
            }
          }
        }

        PanelSeparator { foreground: root.foreground }

        PanelSectionHeader {
          text: strings.t("settings")
          foreground: root.foreground
          fontFamily: root.fontFamily
        }

        Column {
          width: parent.width
          spacing: Style.space(4)
          Text {
            textFormat: Text.PlainText
            color: root.dim
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
            text: strings.t("host")
          }
          Row {
            width: parent.width
            spacing: Style.space(8)
            TextField {
              id: hostField
              width: parent.width - testButton.width - parent.spacing
              foreground: root.foreground
              placeholderText: strings.t("hostPlaceholder")
              text: root.draftHost
              onTextEdited: root.draftHost = text
              Keys.onEscapePressed: root.close()
              onAccepted: root.save()
            }
            Button {
              id: testButton
              text: strings.t("testHost")
              bordered: true
              enabled: !root.busy && root.draftHost.trim() !== ""
              opacity: enabled ? 1 : 0.4
              fontFamily: root.fontFamily
              onClicked: root.testHost()
            }
          }
        }

        Column {
          width: parent.width
          spacing: Style.space(4)
          Text {
            textFormat: Text.PlainText
            color: root.dim
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
            text: strings.t("key")
          }
          Row {
            width: parent.width
            spacing: Style.space(8)
            TextField {
              id: keyField
              width: parent.width - showButton.width - parent.spacing
              foreground: root.foreground
              password: !root.showKey
              placeholderText: root.cfg.keySet ? strings.t("keyPlaceholderSet") : strings.t("keyPlaceholderUnset")
              text: root.draftKey
              onTextEdited: root.draftKey = text
              Keys.onEscapePressed: root.close()
              onAccepted: root.save()
            }
            Button {
              id: showButton
              text: root.showKey ? strings.t("hide") : strings.t("show")
              bordered: true
              fontFamily: root.fontFamily
              onClicked: root.showKey = !root.showKey
            }
          }
        }

        Dropdown {
          width: parent.width
          label: strings.t("fileFetch")
          value: root.draftFileFetch
          foreground: root.foreground
          fontFamily: root.fontFamily
          options: [
            { value: "immediate", label: strings.t("fileFetch.immediate") },
            { value: "on_switch", label: strings.t("fileFetch.on_switch") },
            { value: "off", label: strings.t("fileFetch.off") }
          ]
          onChanged: function(v) { root.draftFileFetch = v }
        }

        Toggle {
          width: parent.width
          label: strings.t("clipboard")
          description: strings.t("clipboardDesc")
          checked: root.draftClipboard
          foreground: root.foreground
          fontFamily: root.fontFamily
          onClicked: root.draftClipboard = !root.draftClipboard
        }

        Text {
          width: parent.width
          visible: root.message !== ""
          textFormat: Text.PlainText
          wrapMode: Text.WordWrap
          color: root.messageIsError ? root.urgent : root.dim
          font.family: root.fontFamily
          font.pixelSize: Style.font.caption
          text: root.message
        }

        Row {
          spacing: Style.space(8)
          Button {
            text: strings.t("save")
            bordered: true
            enabled: root.dirty && !root.busy
            opacity: enabled ? 1 : 0.4
            fontFamily: root.fontFamily
            onClicked: root.save()
          }
        }

        PanelSeparator { foreground: root.foreground }

        // Status: where the service is, and anything still waiting on you.
        Column {
          width: parent.width
          spacing: Style.space(2)

          Text {
            width: parent.width
            textFormat: Text.PlainText
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
            font.bold: true
            text: root.stateLabel()
          }
          Text {
            width: parent.width
            visible: root.dirty
            textFormat: Text.PlainText
            wrapMode: Text.WordWrap
            color: root.dim
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
            text: strings.t("unsaved")
          }
          Text {
            width: parent.width
            visible: root.needsRestart && !root.dirty
            textFormat: Text.PlainText
            wrapMode: Text.WordWrap
            color: root.dim
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
            text: strings.t("needsRestart")
          }
        }
      }
    }
  }
}
