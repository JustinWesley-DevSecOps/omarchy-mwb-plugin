import QtQuick
import qs.Ui

// The bar mark, built from glyphs of the bar's own icon font: two arrows
// pointing at each other and meeting in the middle, with two diamonds nested in
// the V formed by the arrowheads' upper arms. One colour, so it follows the
// bar's foreground.
//
// Each arrow is an open chevron for the head plus a bar for the shaft.
Item {
  id: root

  property color ink: "white"
  property string fontFamily: "JetBrainsMono Nerd Font"

  readonly property real s: Math.min(width, height)
  readonly property real midY: s * 0.5

  // arrowheads: tip on each side of the centre line
  readonly property real headBox: s * 0.56
  readonly property real tipGap: s * 0.015
  // shafts run from the outer edge to the head
  readonly property real shaftBox: s * 0.5
  // The bar font's minus measures higher than it draws; this puts it back on
  // the centre line.
  readonly property real shaftDy: s * 0.0875

  // diamonds, inside the V above the tips
  readonly property real diamondBox: s * 0.16
  readonly property real diamondY: midY - s * 0.235
  readonly property real diamondDx: s * 0.095

  // shafts: 󰍴 minus
  Repeater {
    model: 1
    OpticalGlyph {
      x: 0
      y: root.midY - height / 2 + root.shaftDy
      width: root.shaftBox
      height: root.shaftBox
      text: "󰍴"
      fontFamily: root.fontFamily
      fontSize: 64
      paintedSize: root.shaftBox
      color: root.ink
    }
  }
  Repeater {
    model: 1
    OpticalGlyph {
      x: root.s - width
      y: root.midY - height / 2 + root.shaftDy
      width: root.shaftBox
      height: root.shaftBox
      text: "󰍴"
      fontFamily: root.fontFamily
      fontSize: 64
      paintedSize: root.shaftBox
      color: root.ink
    }
  }

  // head of the left arrow, pointing right: 󰅂 chevron-right
  OpticalGlyph {
    id: headLeft
    x: root.s * 0.5 - root.tipGap - width / 2 - paintedWidth / 2
    y: root.midY - height / 2
    width: root.headBox
    height: root.headBox
    text: "󰅂"
    fontFamily: root.fontFamily
    fontSize: 64
    paintedSize: root.headBox
    color: root.ink
  }

  // head of the right arrow, pointing left: 󰅁 chevron-left
  OpticalGlyph {
    id: headRight
    x: root.s * 0.5 + root.tipGap - width / 2 + paintedWidth / 2
    y: root.midY - height / 2
    width: root.headBox
    height: root.headBox
    text: "󰅁"
    fontFamily: root.fontFamily
    fontSize: 64
    paintedSize: root.headBox
    color: root.ink
  }

  // two diamonds: 󰨐
  Repeater {
    model: [-1, 1]
    OpticalGlyph {
      required property int modelData
      x: root.s * 0.5 + modelData * root.diamondDx - width / 2
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
