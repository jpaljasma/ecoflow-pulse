EcoFlow Delta Assets Manifest

manifest.json maps product slugs to relative paths inside this zip for:
- original: PNG (converted from CDN response; preserves transparency)
- original_webp: original download as served
- 1024/512/256: center-cropped square PNGs

Example:
  { "delta_pro_ultra": { "original": ".../original_png/delta_pro_ultra.png", "1024": ".../crop_1024/delta_pro_ultra_1024.png", ... } }

All paths are relative to the zip root.
