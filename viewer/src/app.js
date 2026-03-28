import * as THREE from 'three';
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';

const container = document.getElementById('viewer-container');
const errorMessage = document.getElementById('errorMessage');
const loadingIndicator = document.getElementById('loadingIndicator');
const fileInput = document.getElementById('fileInput');

const scene = new THREE.Scene();
scene.background = new THREE.Color(0x20232a);

const camera = new THREE.PerspectiveCamera(45, window.innerWidth / window.innerHeight, 0.1, 1000);
camera.position.set(0, 1.5, 3);

const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false });
renderer.setSize(window.innerWidth, window.innerHeight);
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
container.appendChild(renderer.domElement);

const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping = false;
controls.minDistance = 0.5;
controls.maxDistance = 10;
controls.target.set(0, 0.8, 0);
controls.update();

const ambientLight = new THREE.AmbientLight(0xffffff, 0.6);
scene.add(ambientLight);

const directionalLight = new THREE.DirectionalLight(0xffffff, 0.8);
directionalLight.position.set(5, 10, 7.5);
scene.add(directionalLight);

const loader = new GLTFLoader();
const animationSelect = document.getElementById('animationSelect');
const animationLabel = document.getElementById('animationLabel');

const clock = new THREE.Clock();
let currentModel = null;
let mixer = null;
let activeAction = null;
let currentAnimations = [];
let activeBlobUrl = null;

function showError(message) {
  if (!message) {
    errorMessage.textContent = '';
    return;
  }
  errorMessage.textContent = message;
  console.error(message);
}

function showLoading(message) {
  loadingIndicator.textContent = message || '';
}

function clearModel() {
  if (mixer) {
    mixer.stopAllAction();
    mixer = null;
    activeAction = null;
  }

  if (!currentModel) return;

  scene.remove(currentModel);
  currentModel.traverse((child) => {
    if (child.isMesh) {
      child.geometry.dispose();
      if (child.material) {
        if (Array.isArray(child.material)) {
          child.material.forEach(m => m.dispose());
        } else {
          child.material.dispose();
        }
      }
    }
  });
  currentModel = null;
}

function focusModel(model) {
  const box = new THREE.Box3().setFromObject(model);
  const size = box.getSize(new THREE.Vector3());
  const center = box.getCenter(new THREE.Vector3());

  const maxDim = Math.max(size.x, size.y, size.z);
  const fov = camera.fov * (Math.PI / 180);
  let cameraZ = Math.abs(maxDim / 2 / Math.tan(fov / 2));
  cameraZ *= 1.5;

  camera.position.set(center.x, center.y + maxDim * 0.2, center.z + cameraZ);
  camera.lookAt(center);

  const minZ = box.min.z;
  const cameraToFarEdge = (minZ < 0) ? -minZ + cameraZ : cameraZ - minZ;
  camera.far = cameraToFarEdge * 3;
  camera.updateProjectionMatrix();

  controls.target.copy(center);
  controls.update();
}

function isSupportedExtension(url) {
  const low = url.toLowerCase();
  if (low.startsWith('blob:') || low.startsWith('data:')) {
    // Local file blobs may not include extension in URL.
    return true;
  }
  return low.endsWith('.glb') || low.endsWith('.gltf');
}

function setAnimationOptions(animations) {
  animationSelect.innerHTML = '';

  if (!animations || animations.length === 0) {
    animationLabel.hidden = true;
    return;
  }

  animationLabel.hidden = false;

  animations.forEach((clip, index) => {
    const option = document.createElement('option');
    option.value = String(index);
    option.textContent = clip.name || `Animation ${index + 1}`;
    animationSelect.appendChild(option);
  });

  animationSelect.value = '0';
}

function loadModel(url) {
  showError('');
  showLoading('Loading ...');

  if (!url || typeof url !== 'string') {
    showError('Invalid file URL.');
    showLoading('');
    return;
  }

  const trimmedUrl = url.trim();
  if (!isSupportedExtension(trimmedUrl)) {
    showError('Only .gltf and .glb files are supported.');
    showLoading('');
    return;
  }

  if (activeBlobUrl && activeBlobUrl !== trimmedUrl) {
    URL.revokeObjectURL(activeBlobUrl);
    activeBlobUrl = null;
  }

  loader.load(
    trimmedUrl,
    (gltf) => {
      clearModel();

      currentModel = gltf.scene;
      scene.add(currentModel);

      if (gltf.animations && gltf.animations.length > 0) {
        currentAnimations = gltf.animations;
        mixer = new THREE.AnimationMixer(currentModel);
        setAnimationOptions(gltf.animations);

        activeAction = mixer.clipAction(currentAnimations[0]);
        activeAction.reset().play();
      } else {
        currentAnimations = [];
        setAnimationOptions([]);
      }

      focusModel(currentModel);
      showLoading('');
    },
    (xhr) => {
      if (xhr.lengthComputable) {
        const progress = ((xhr.loaded / xhr.total) * 100).toFixed(1);
        showLoading(`Loading ${progress}%`);
      } else {
        showLoading('Loading...');
      }
    },
    (error) => {
      showError(`Failed to load model: ${error.message || error}`);
      showLoading('');
    }
  );
}

fileInput.addEventListener('change', async (event) => {
  const file = event.target.files?.[0];
  if (!file) return;

  const extension = file.name.split('.').pop()?.toLowerCase();
  if (!['gltf', 'glb'].includes(extension ?? '')) {
    showError('Only .gltf and .glb files are supported for local upload.');
    showLoading('');
    return;
  }

  showError('');
  showLoading('Preparing local file...');

  if (activeBlobUrl) {
    URL.revokeObjectURL(activeBlobUrl);
    activeBlobUrl = null;
  }

  activeBlobUrl = URL.createObjectURL(file);
  loadModel(activeBlobUrl);
});

animationSelect.addEventListener('change', () => {
  if (!mixer || !currentAnimations.length) return;

  const idx = Number(animationSelect.value);
  if (Number.isNaN(idx) || idx < 0 || idx >= currentAnimations.length) return;

  if (activeAction) {
    activeAction.stop();
  }

  const clip = currentAnimations[idx];
  if (!clip) return;

  activeAction = mixer.clipAction(clip);
  activeAction.reset().play();
});

window.addEventListener('resize', () => {
  camera.aspect = window.innerWidth / window.innerHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(window.innerWidth, window.innerHeight);
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
});

function animate() {
  requestAnimationFrame(animate);

  const delta = clock.getDelta();
  if (mixer) {
    mixer.update(delta);
  }

  controls.update();
  renderer.render(scene, camera);
}

animate();
