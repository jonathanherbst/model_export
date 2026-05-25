# Model Exporter

The goal of this project is to make a program that can extract a character model from a game to a single binary GLTF file (.glb) with all the textures, meshes, animations, and configuration options built in.  Starting with WoW, but might be expanded to other games in the future.

**_NOTE:_** This project is not meant to be a general purpose app for exploring wow models, [wow.export](https://www.kruithne.net/wow.export/) is a fantastic project if that's what you're looking for.

## How to Run

To build the extractor program you need golang https://go.dev/doc/install, cmake, and libbz2.  I've included a devcontainer which contains the correct versions of everything you need to build and run.

``` sh
git clone --recursive https://github.com/jonathanherbst/model_export.git
go generate ./...

# list races
go run . export --casc <path_to_wow>

# export a character
go run . export --casc <path_to_wow> --glb <path_to_glb_file> "<character_race>" <body_type:0|1|m|f>

# example: export a night elf female from wow install at /opt/wow to a glb file called nelf.glb
go run . export --casc /opt/wow --glb nelf.glb "Night Elf" f

# there are other tools for exploring the wow database and individual files
go run . --help
```

To be able to get everything the application needs from a WoW install it has to download some files from the internet, which it does automatically.  Everything the application downloads gets written to the working directory when you run the application.

- wow-listfile.csv: The latest listfile from https://github.com/wowdev/wow-listfile/, needed to find the correct database files in your local wow install (casc).
- wow-dbd.zip: The latest database definitions from https://github.com/wowdev/WoWDBDefs, needed to parse the database files in your local wow install (casc).

The exported glb files use a custom gltf extension that's not supported by any other programs so the project has a ui to handle the extension properly.  It just requires nodejs to run https://nodejs.org/en/download, dependencies are also included in the devcontainer so you can run it that way too.

``` sh
cd viewer

# hosts a web interface at localhost:1234
npm run start
```
