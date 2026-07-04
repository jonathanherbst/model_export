#version 300 es
precision highp float;

in vec2 vUv;

uniform sampler2D uBase;
uniform sampler2D uOverlay;
uniform vec2 uCanvasSize;
uniform vec2 uSegmentPos;
uniform vec2 uSegmentSize;

out vec4 fragColor;

void main() {
    vec4 base = texture(uBase, vUv);
    vec2 overlayUv = (vUv * uCanvasSize - uSegmentPos) / uSegmentSize;
    vec4 overlay = texture(uOverlay, overlayUv);
    fragColor.rgb = overlay.rgb * overlay.a + base.rgb * (1.0 - overlay.a);
    fragColor.a = overlay.a + base.a * (1.0 - overlay.a);
}
