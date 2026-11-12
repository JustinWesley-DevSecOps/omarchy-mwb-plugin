import QtQuick
import qs.Ui

// The bar mark, built from glyphs of the bar's own icon font: two arrows
// pointing at each other and meeting in the middle, with two small diamonds
// above the meeting point. One colour, so it follows the bar's foreground.
//
// The font's arrows are thin, so each is drawn several times a hair apart to
// thicken it, the way a glyph is emboldened.
Item {
  id: root

  property color ink: "white"
  property string fontFamily: "JetBrainsMono Nerd Font"

  readonly property real s: Math.min(width, height)

  readonly property real arrowBox: s * 0.5
  readonly property real arrowY: s * 0.64
  readonly property real weight: s * 0.032
  readonly property var offsets: [
    [0, 0], [-1, 0], [1, 0], [0, -1], [0, 1],
    [-0.7, -0.7], [0.7, -0.7], [-0.7, 0.7], [0.7, 0.7]
  ]

  readonly property real diamondBox: s * 0.27
  readonly property real diamondY: s * 0.22

  // 󰁔 arrow-right, coming in from the left
  Repeater {
    model: root.offsets
    OpticalGlyph {
      required property var modelData
      x: root.s * 0.5 - width + modelData[0] * root.weight
      y: root.arrowY - height / 2 + modelData[1] * root.weight
      width: root.arrowBox
      height: root.arrowBox
      text: "󰁔"
      fontFamily: root.fontFamily
      fontSize: 64
      paintedSize: root.arrowBox
      color: root.ink
    }
  }

  // 󰁍 arrow-left, coming in from the right
  Repeater {
    model: root.offsets
    OpticalGlyph {
      required property var modelData
      x: root.s * 0.5 + modelData[0] * root.weight
      y: root.arrowY - height / 2 + modelData[1] * root.weight
      width: root.arrowBox
      height: root.arrowBox
      text: "󰁍"
      fontFamily: root.fontFamily
      fontSize: 64
      paintedSize: root.arrowBox
      color: root.ink
    }
  }

  // 󰨐 two diamonds above the meeting point
  Repeater {
    model: [0.38, 0.62]
    OpticalGlyph {
      required property real modelData
      x: root.s * modelData - width / 2
      y: root.diamondY - height / 2
      width: root.diamondBox
      height: root.diamondBox
      text: "󰨐"
      fontFamily: root.fontFamily
      fontSize: 64
      paintedSize: root.diamondBox
      color: root.ink
    }
  }
}
