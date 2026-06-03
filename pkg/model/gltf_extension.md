# GLTF Extension

Extension needs to support the configuration options.
- Turning on and off geosets
- Turning on and off sections of textures
- Some configuration options combine to decide if texture sections should be turned on or off
- Texture sections have different blending 

## MDLE_SegmentedTexture

This extension gets applied to a Material.  It defines that the material is made up of segments that get applied over each other to build the material.  A material defines _width_ and _height_ indicating the size of the material in pixels.  Each segment defines an area of the material that the segment is applied to, _x_ and _y_ define the coordinates of the top left corner of the area, and _width_ and _height_ define the extenst of the area from the top left corner.  Segments get applied in the order they are in the segment list, and are blended into the prevoius segments using the mode defined in the *blend_mode* field.  Images are bound to segments from the *MDLE_Configuration* elements.

```json
{
  "width": 2048,
  "height": 1024,
  "segments": [
    {
      "x": 0,
      "y": 0,
      "width": 1024,
      "height": 1024,
      "blend_mode": "overlay",
    }
  ]
}
```

Materials should have a default texture through the PBRMetallicRoughness field.  When building a new texture from the configuration options it should replace default texture, but keep the same sampler, and material configuration.

### Blend Modes

- **_overlay_**: the segment gets overlayed on top of the texture constructed thus far, blending by alpha channel.

## MDLE_Configuration

This extension gets added to the document to define all the configuration choices for the scene and how they map to geosets and materials.  Choices have an option name which is shared between related choices.  Choices within an option are uniquely identified by the choice name and/or the color fields.  You can think of choices as an html _select_ element except option name is the label and choices are the individual options.

Elements define how choices map to geosets and materials.  Each element has a list of choice indices within the choices list, all of which should be selected by the UI for the element to be applied.  Applying the element requires enabling all the materials defined in the materials list and the mesh ids defined in the meshes list.  A material has an index into the document materials array which points to a segmented texture material, the segment field defines an index into the material's segments array, and the image field defines an index into the document's image list to be applied to the material using the segment definition.

Static meshes are meshes that aren't affected by configurations, what this does is help the renderer figure out what meshes to enable before applying configurations.

```json
{
  "choices": [
    {
      "option": "<option_name>",
      "choice": "<choice_name>",
      "color": 0x01234567
    }
  ],
  "elements": [
    {
      "choices": [0],
      "materials": [
        {
          "material": 0,
          "segment": 2,
          "image": 50
        }
      ],
      "meshes": [0]
    }
  ],
  "static_meshes": [0, 1]
}
```