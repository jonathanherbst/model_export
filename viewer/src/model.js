import * as THREE from 'three';
import { GLTF, GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';

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

    /** @type {Set<number>} */
    #static_meshes;

    /** @type {*} */
    #combiners;

    /**
     * @param {GLTF} gltf 
     */
    constructor(gltf) {
        this.#gltf = new ModelExportGLTF(gltf)
        this.#combiners = this.#gltf.load_combiners()
        this.configurations = this.#gltf.load_config()
        this.#segmented_textures = this.#gltf.load_segmented_textures(this.#combiners)
        this.#config_elements = this.#gltf.load_config_elements()

        this.#config_meshes = new Set()
        this.#config_elements.forEach((element) => {
            element.meshes.forEach((mesh) => {
                this.#config_meshes.add(mesh)
            })
        })
        this.#static_meshes = this.#gltf.load_static_meshes()
    }

    get scene() {
        return this.#gltf.gltf.scene
    }

    get animations() {
        return this.#gltf.gltf.animations
    }

    /**
     * @returns {{ name: string, canvas: HTMLCanvasElement, width: number, height: number }[]}
     */
    getMaterials() {
        const result = []
        this.#gltf.gltf.parser.json.materials.forEach((mat, i) => {
            if(mat.extensions?.MDLE_SegmentedTexture) {
                const tex = this.#segmented_textures.get(i)
                if(tex) {
                    result.push({
                        name: tex.name || `Material ${i}`,
                        canvas: tex.canvas,
                        width: tex.canvas.width,
                        height: tex.canvas.height,
                    })
                }
            }
        })
        return result
    }

    /**
     * @param {Set<number>} enabled_choices 
     */
    async set_enabled_choices(enabled_choices) {
        this.#enabled_choices = enabled_choices

        // reset all the meshes
        for(let [meshIdx, mesh] of this.#gltf.mesh_map) {
            if(this.#static_meshes.has(meshIdx)) {
                mesh.visible = true;
            } else {
                mesh.visible = false;
            }
        }

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
    /** @type {Map<number, THREE.Mesh>} */
    mesh_map;

    /**
     * @param {GLTF} gltf 
     */
    constructor(gltf) {
        this.gltf = gltf
        
        this.material_map = new Map()
        this.mesh_map = new Map()
        for(const [three_obj, gltf_obj] of gltf.parser.associations) {
            if(gltf_obj?.materials !== undefined && three_obj.isMaterial) {
                let list = this.material_map.get(gltf_obj.materials)
                if(list) {
                    list.push(three_obj)
                } else {
                    this.material_map.set(gltf_obj.materials, [three_obj])
                }
            }

            if(gltf_obj?.meshes !== undefined && three_obj.isObject3D) {
                console.assert(!this.mesh_map.has(gltf_obj.meshes))
                this.mesh_map.set(gltf_obj.meshes, three_obj)
            }
        }
    }

    /**
     * @returns {Map<string, {{id: number, name: string, color: number}}>}
     */
    load_combiners() {
        const sceneExt = this.gltf.userData.gltfExtensions?.MDLE_Scene
        if(!sceneExt?.combiners || !sceneExt?.shaders) return null
        return { shaders: sceneExt.shaders, combiners: sceneExt.combiners }
    }

    load_config() {
        let configurations = new Map();
        const ext_configs = this.gltf.userData.gltfExtensions?.MDLE_Scene
        if(ext_configs) {
            for(let i = 0; i < ext_configs.choices.length; ++i) {
                let choice = ext_configs.choices[i]
                let option = {
                    id: i,
                    name: choice.choice,
                    color: choice.color,
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
        const ext_configs = this.gltf.userData.gltfExtensions?.MDLE_Scene
        if(ext_configs) {
            for(let element of ext_configs.elements) {
                const meshes = element.meshes.map((mesh_idx) => {
                    const mesh = this.mesh_map.get(mesh_idx)
                    if(!mesh) {
                        console.error("element references a nonexistent mesh id", element)
                    }
                    return mesh
                })
                elements.push({
                    choices: element.choices,
                    materials: element.materials,
                    meshes: meshes,
                })
            }
        }
        return Object.freeze(elements)
    }

    /**
     * @returns {Map<number, SegmentedTexture>}
     */
    load_segmented_textures(combiners) {
        let textures = new Map()
        this.gltf.parser.json.materials.forEach((mat, i) => {
            if(mat.extensions?.MDLE_SegmentedTexture && this.material_map.has(i)) {
                let three_mats = this.material_map.get(i)
                const texture = new SegmentedTexture(three_mats[0], combiners)
                for(let mat of three_mats) {
                    mat.map = texture.texture
                }

                textures.set(i, texture)
            }
        })
        return textures
    }

    /**
     * @returns {Set<number>}
     */
    load_static_meshes() {
        const ext_configs = this.gltf.userData.gltfExtensions?.MDLE_Scene
        if(ext_configs) {
            return new Set(ext_configs.static_meshes)
        }
        return new Set()
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

class SegmentedTexture {
    /** @type {string} */
    name;

    /** @type {THREE.Texture} */
    texture;

    /** @type {HTMLCanvasElement} */
    #canvas;

    /** @type {TextureSegment[]} */
    #segments;

    /** @type {WebGL2RenderingContext|null} */
    #gl;

    /** @type {{shaders: object, combiners: object}|null} */
    #shaderData;

    /** @type {Map<string, WebGLProgram>|null} */
    #programs;

    /** @type {Map<WebGLProgram, object>} */
    #uniforms;

    /** @type {WebGLFramebuffer|null} */
    #fbo0;

    /** @type {WebGLFramebuffer|null} */
    #fbo1;

    /** @type {WebGLTexture|null} */
    #tex0;

    /** @type {WebGLTexture|null} */
    #tex1;

    /** @type {WebGLVertexArrayObject|null} */
    #vao;

    /** @type {Map<number, {texture: WebGLTexture, image: ImageBitmap}>} */
    #segmentTexCache;

    get canvas() {
        return this.#canvas
    }

    /**
     * @param {THREE.Material} material
     * @param {{shaders: object, combiners: object}|null} shaderData
     */
    constructor(material, shaderData) {
        this.name = material.name
        const segTex = material.userData.gltfExtensions.MDLE_SegmentedTexture
        this.#segments = segTex.segments
        this.#canvas = document.createElement("canvas")
        this.#canvas.height = segTex.height
        this.#canvas.width = segTex.width
        this.#shaderData = shaderData
        this.#programs = null
        this.#segmentTexCache = new Map()

        this.texture = new THREE.CanvasTexture(this.#canvas)
        const srcMap = material.map
        this.texture.mapping = srcMap.mapping
        this.texture.wrapS = srcMap.wrapS
        this.texture.wrapT = srcMap.wrapT
        this.texture.magFilter = srcMap.magFilter
        this.texture.minFilter = srcMap.minFilter
        this.texture.channel = srcMap.channel
        this.texture.flipY = srcMap.flipY
        this.texture.generateMipmaps = srcMap.generateMipmaps
        this.texture.colorSpace = srcMap.colorSpace
        this.texture.anisotropy = srcMap.anisotropy
        this.texture.offset = srcMap.offset
        this.texture.repeat = srcMap.repeat
        this.texture.rotation = srcMap.rotation
        this.texture.needsUpdate = true

        this.#initWebGL()
    }

    #initWebGL() {
        if(!this.#shaderData) return

        const gl = this.#canvas.getContext("webgl2", { antialias: false })
        if(!gl) return
        this.#gl = gl

        const shaderData = this.#shaderData

        // compile programs
        this.#programs = new Map()
        for(const [name, combiner] of Object.entries(shaderData.combiners)) {
            const vsSrc = shaderData.shaders[combiner.vertex]
            const fsSrc = shaderData.shaders[combiner.fragment]
            if(!vsSrc || !fsSrc) {
                console.warn(`missing shader for combiner: ${name}`)
                continue
            }

            const vs = gl.createShader(gl.VERTEX_SHADER)
            gl.shaderSource(vs, vsSrc)
            gl.compileShader(vs)
            if(!gl.getShaderParameter(vs, gl.COMPILE_STATUS)) {
                console.error(`vertex shader compile error (${name}):`, gl.getShaderInfoLog(vs))
                gl.deleteShader(vs)
                continue
            }

            const fs = gl.createShader(gl.FRAGMENT_SHADER)
            gl.shaderSource(fs, fsSrc)
            gl.compileShader(fs)
            if(!gl.getShaderParameter(fs, gl.COMPILE_STATUS)) {
                console.error(`fragment shader compile error (${name}):`, gl.getShaderInfoLog(fs))
                gl.deleteShader(vs)
                gl.deleteShader(fs)
                continue
            }

            const program = gl.createProgram()
            gl.attachShader(program, vs)
            gl.attachShader(program, fs)
            gl.linkProgram(program)
            if(!gl.getProgramParameter(program, gl.LINK_STATUS)) {
                console.error(`program link error (${name}):`, gl.getProgramInfoLog(program))
                gl.deleteProgram(program)
                gl.deleteShader(vs)
                gl.deleteShader(fs)
                continue
            }

            gl.deleteShader(vs)
            gl.deleteShader(fs)
            this.#programs.set(name, program)
        }

        // cache uniform locations for each program
        this.#uniforms = new Map()
        for(const [name, program] of this.#programs) {
            this.#uniforms.set(program, {
                uBase: gl.getUniformLocation(program, "uBase"),
                uOverlay: gl.getUniformLocation(program, "uOverlay"),
                uCanvasSize: gl.getUniformLocation(program, "uCanvasSize"),
                uSegmentPos: gl.getUniformLocation(program, "uSegmentPos"),
                uSegmentSize: gl.getUniformLocation(program, "uSegmentSize"),
            })
        }

        // create ping-pong FBOs
        const w = this.#canvas.width
        const h = this.#canvas.height

        const makeTex = () => {
            const tex = gl.createTexture()
            gl.bindTexture(gl.TEXTURE_2D, tex)
            gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, null)
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
            return tex
        }

        const makeFbo = (tex) => {
            const fbo = gl.createFramebuffer()
            gl.bindFramebuffer(gl.FRAMEBUFFER, fbo)
            gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
            return fbo
        }

        this.#tex0 = makeTex()
        this.#tex1 = makeTex()
        this.#fbo0 = makeFbo(this.#tex0)
        this.#fbo1 = makeFbo(this.#tex1)

        // fullscreen quad VAO
        const positions = new Float32Array([
            -1, -1,  1, -1, -1,  1,
            -1,  1,  1, -1,  1,  1,
        ])
        const uvs = new Float32Array([
            0, 0,  1, 0,  0, 1,
            0, 1,  1, 0,  1, 1,
        ])

        this.#vao = gl.createVertexArray()
        gl.bindVertexArray(this.#vao)

        const posBuf = gl.createBuffer()
        gl.bindBuffer(gl.ARRAY_BUFFER, posBuf)
        gl.bufferData(gl.ARRAY_BUFFER, positions, gl.STATIC_DRAW)
        gl.enableVertexAttribArray(0)
        gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0)

        const uvBuf = gl.createBuffer()
        gl.bindBuffer(gl.ARRAY_BUFFER, uvBuf)
        gl.bufferData(gl.ARRAY_BUFFER, uvs, gl.STATIC_DRAW)
        gl.enableVertexAttribArray(1)
        gl.vertexAttribPointer(1, 2, gl.FLOAT, false, 0, 0)

        gl.bindVertexArray(null)
    }

    /**
     * @param {Map<number, ImageBitmap>} images
     */
    set_segments(images) {
        const gl = this.#gl
        if(!gl) return

        const w = this.#canvas.width
        const h = this.#canvas.height

        gl.viewport(0, 0, w, h)

        // clear accumulator (fbo0) to transparent black
        gl.bindFramebuffer(gl.FRAMEBUFFER, this.#fbo0)
        gl.clearColor(0, 0, 0, 0)
        gl.clear(gl.COLOR_BUFFER_BIT)

        for(let i = 0; i < this.#segments.length; i++) {
            const img = images.get(i)
            if(!img) continue

            const segment = this.#segments[i]
            const program = this.#programs?.get(segment.combiner)
            if(!program) continue

            // upload or reuse segment texture
            let texEntry = this.#segmentTexCache.get(i)
            if(texEntry && texEntry.image !== img) {
                gl.deleteTexture(texEntry.texture)
                texEntry = null
            }
            if(!texEntry) {
                const tex = gl.createTexture()
                gl.bindTexture(gl.TEXTURE_2D, tex)
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img)
                gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
                gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
                gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
                gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
                texEntry = { texture: tex, image: img }
                this.#segmentTexCache.set(i, texEntry)
            }

            const uniforms = this.#uniforms.get(program)

            gl.useProgram(program)

            // copy accumulator (fbo0) to temp (fbo1) for safe reading
            gl.bindFramebuffer(gl.READ_FRAMEBUFFER, this.#fbo0)
            gl.bindFramebuffer(gl.DRAW_FRAMEBUFFER, this.#fbo1)
            gl.blitFramebuffer(0, 0, w, h, 0, 0, w, h, gl.COLOR_BUFFER_BIT, gl.NEAREST)

            // read base from temp (tex1), write composited result to accumulator (fbo0)
            gl.bindFramebuffer(gl.FRAMEBUFFER, this.#fbo0)
            gl.activeTexture(gl.TEXTURE0)
            gl.bindTexture(gl.TEXTURE_2D, this.#tex1)
            gl.uniform1i(uniforms.uBase, 0)

            // uOverlay = segment image
            gl.activeTexture(gl.TEXTURE1)
            gl.bindTexture(gl.TEXTURE_2D, texEntry.texture)
            gl.uniform1i(uniforms.uOverlay, 1)

            gl.uniform2f(uniforms.uCanvasSize, w, h)
            gl.uniform2f(uniforms.uSegmentPos, segment.x, segment.y)
            gl.uniform2f(uniforms.uSegmentSize, segment.width, segment.height)

            // render to accumulator with scissor
            gl.enable(gl.SCISSOR_TEST)
            gl.scissor(segment.x, segment.y, segment.width, segment.height)

            gl.bindVertexArray(this.#vao)
            gl.drawArrays(gl.TRIANGLES, 0, 6)
            gl.bindVertexArray(null)

            gl.disable(gl.SCISSOR_TEST)
        }

        // blit final accumulator to canvas (Y-flipped for top-left canvas pixel order)
        gl.bindFramebuffer(gl.READ_FRAMEBUFFER, this.#fbo0)
        gl.bindFramebuffer(gl.DRAW_FRAMEBUFFER, null)
        gl.blitFramebuffer(0, 0, w, h, 0, h, w, 0, gl.COLOR_BUFFER_BIT, gl.NEAREST)

        this.texture.needsUpdate = true
    }
}
