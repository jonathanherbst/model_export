import * as THREE from 'three';
import { GLTF, GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';
import { element, thickness } from 'three/src/nodes/TSL.js';

/**
 * Load a gltf file from a url
 * @param {string} url
 * @param {function(ProgressEvent): void} [on_progress]
 * @returns {Model}
 */
export async function load_gltf(url, on_progress) {
    const loader = new GLTFLoader()
    const gltf = await loader.loadAsync(url, on_progress)
    return new Model(gltf)
}

/**
 * @typedef {Object} ConfigElementMaterial
 * @property {number} material ID of a segmented texture
 * @property {number} segment  Index of the segment within the segmented texture
 * @property {number} image    Index of the image to apply to the segmented texture
 */

/**
 * @typedef {Object} ConfigElement
 * @property {number[]} choices IDs of all the confiugrations that must be enabled to enable this element
 * @property {ConfigElementMaterial[]} materials All of the materials to enable when this element is enabled
 * @property {THREE.Mesh[]} meshes All of the meshes to enable when this element is enabled
 */

export class Model {
    /** @type {ModelExportGLTF} */
    #gltf;

    /** @type {Map<string, {{id: number, name: string, color: number}}>} */
    configurations;

    /** @type {Set<number>} */
    #enabled_choices;

    /** @type {Map<number, SegmentedTexture>} */
    #segmented_textures;

    /** @type {ConfigElement[]} */
    #config_elements;

    /** @type {Set<THREE.Mesh>} */
    #config_meshes;

    /**
     * @param {GLTF} gltf 
     */
    constructor(gltf) {
        this.#gltf = new ModelExportGLTF(gltf)
        this.configurations = this.#gltf.load_config()
        this.#segmented_textures = this.#gltf.load_segmented_textures()
        this.#config_elements = this.#gltf.load_config_elements()

        this.#config_meshes = new Set()
        this.#config_elements.forEach((element) => {
            element.meshes.forEach((mesh) => {
                this.#config_meshes.add(mesh)
            })
        })
    }

    get scene() {
        return this._gltf.scene
    }

    get animations() {
        return this._gltf.animations
    }

    /**
     * @param {Set<number>} enabled_choices 
     */
    async set_enabled_choices(enabled_choices) {
        this.#enabled_choices = enabled_choices

        // disable all the meshes
        this.#config_meshes.forEach((mesh) => {
            mesh.visible = false
        })

        let texture_configs = new Map()
        for(let element of this.#config_elements) {
            let enabled = true
            element.choices.forEach((choice) => {
                enabled = enabled && enabled_choices.has(choice)
            })
            if(enabled) {
                // enable this elements meshes
                element.meshes.forEach((mesh) => mesh.visible = true)

                // load all the images for the segmented textures
                for(let mat of element.materials) {
                    let img = await this.#gltf.load_image(mat.image)
                    if(texture_configs.has(mat.material)) {
                        texture_configs.get(mat.material).set(mat.segment, img)
                    } else {
                        let tex_config = new Map()
                        tex_config.set(mat.segment, img)
                        texture_configs.set(mat.material, tex_config)
                    }
                }
            }
        }

        for(let [mat_idx, tex_config] of texture_configs) {
            this.#segmented_textures.get(mat_idx).set_segments(tex_config)
        }
    }
}

class ModelExportGLTF {
    /**
     * @param {GLTF} gltf 
     */
    constructor(gltf) {
        this.gltf = gltf
    }

    /**
     * @returns {Map<string, {{id: number, name: string, color: number}}>}
     */
    load_config() {
        let configurations = new Map();
        const ext_configs = this.gltf.userData.gltfExtensions?.MDLE_Configuration
        if(ext_configs) {
            for(let i = 0; i < ext_configs.choices.length; ++i) {
                let choice = ext_configs.choices[i]
                let option = {
                    id: i,
                    name: choice.option,
                    color: choice.choice,
                }
                if(configurations.has(choice.option)) {
                    configurations.get(choice.option).push(option)
                } else {
                    configurations.set(choice.option, [option])
                }
            }
        }
        return configurations
    }

    
    load_config_elements() {
        let elements = new Array()
        const ext_configs = this.gltf.userData.gltfExtensions?.MDLE_Configuration
        if(ext_configs) {
            for(element of ext_configs.elements) {
                elements.push({
                    choices: element.choices,
                    materials: element.materials,
                    meshes: element.meshes.map((mesh_idx) => {
                        const mesh_name = this.gltf.parser.json.meshes[mesh_idx].name
                        return this.gltf.scene.getObjectByName(mesh_name)
                    })
                })
            }
        }
        return Object.freeze(elements)
    }

    /**
     * @returns {Map<number, SegmentedTexture>}
     */
    load_segmented_textures() {
        let textures = new Map()
        this.gltf.parser.json.materials.forEach((mat, i) => {
            if(mat.extensions?.MDLE_SegmentedTexture) {
                let material = this.gltf.scene.getObjectByName(mat.name)
                textures.set(i, new SegmentedTexture(material))
            }
        })
        return textures
    }

    /**
     * Load an image from the file by index into the gltf image array
     * @param {number} idx 
     * @returns {ImageBitmap}
     */
    async load_image(idx) {
        return (await this.gltf.parser.loadImageSource(idx, this.gltf.parser.textureLoader)).image
    }
}

/**
 * @typedef {Object} TextureSegment
 * @property {number} x
 * @property {number} y
 * @property {number} width
 * @property {number} height
 * @property {string} overlay
 */

class SegmentedTexture {
    /** @type {string} */
    name;

    /** @type {THREE.Texture} */
    texture;

    /** @type {HTMLCanvasElement} */
    #canvas;

    /** @type {TextureSegment[]} */
    #segments;

    /**
     * @param {THREE.Material} material 
     */
    constructor(material) {
        this.name = material.name
        this.#segments = material.userData.gltfExtensions.MDLE_SegmentedTexture.segments
        this.#canvas = document.createElement("canvas")
        this.#canvas.height = material.userData.gltfExtensions.MDLE_SegmentedTexture.height
        this.#canvas.width = material.userData.gltfExtensions.MDLE_SegmentedTexture.height

        this.texture = new THREE.CanvasTexture(
            this.#canvas,
            material.map.mapping,
            material.map.wrapS,
            material.map.wrapT,
            material.map.magFilter,
            material.map.minFilter,
        )
    }

    /**
     * @param {Map<number, ImageBitmap>} images 
     */
    set_segments(images) {
        let ctx = this.#canvas.getContext("2d")
        this.#segments.forEach((segment, i) => {
            let img = images.get(i)
            if(img) {
                ctx.drawImage(img, segment.x, segment.y, segment.width, segment.height)
            }
        })
    }
}
