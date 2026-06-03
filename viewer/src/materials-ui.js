const VIEW_SIZE = 256;

export function setupMaterialsPanel(model, { onClose } = {}) {
  const backdrop = document.createElement('div');
  backdrop.id = 'materials-modal-backdrop';
  backdrop.className = 'materials-modal-backdrop';
  backdrop.hidden = true;

  const dialog = document.createElement('div');
  dialog.className = 'materials-modal-dialog';
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-label', 'Materials');

  const header = document.createElement('div');
  header.className = 'materials-modal-header';
  const title = document.createElement('span');
  title.className = 'materials-modal-title';
  title.textContent = 'Materials';
  const closeBtn = document.createElement('button');
  closeBtn.type = 'button';
  closeBtn.className = 'materials-modal-close';
  closeBtn.setAttribute('aria-label', 'Close');
  closeBtn.textContent = '×';
  header.appendChild(title);
  header.appendChild(closeBtn);
  dialog.appendChild(header);

  const grid = document.createElement('div');
  grid.className = 'materials-grid';
  dialog.appendChild(grid);

  backdrop.appendChild(dialog);
  document.body.appendChild(backdrop);

  let viewCanvases = [];
  let rafId = null;
  let isOpen = false;
  let keyHandler = null;

  function syncFrame() {
    rafId = null;
    if (!isOpen) return;
    for (const { view, src } of viewCanvases) {
      const ctx = view.getContext('2d');
      ctx.clearRect(0, 0, view.width, view.height);
      if (src && src.width > 0 && src.height > 0) {
        ctx.drawImage(src, 0, 0, src.width, src.height, 0, 0, view.width, view.height);
      }
    }
    rafId = requestAnimationFrame(syncFrame);
  }

  function buildGrid() {
    while (grid.firstChild) grid.removeChild(grid.firstChild);
    viewCanvases = [];

    const materials = model.getMaterials();
    if (materials.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'materials-empty';
      empty.textContent = 'This model has no segmented materials.';
      grid.appendChild(empty);
      return;
    }

    for (const mat of materials) {
      const card = document.createElement('div');
      card.className = 'material-card';

      const name = document.createElement('div');
      name.className = 'material-card-name';
      name.textContent = mat.name;
      card.appendChild(name);

      const wrapper = document.createElement('div');
      wrapper.className = 'material-checker';
      const canvas = document.createElement('canvas');
      canvas.className = 'material-canvas';
      canvas.width = VIEW_SIZE;
      canvas.height = VIEW_SIZE;
      wrapper.appendChild(canvas);
      card.appendChild(wrapper);

      const dims = document.createElement('div');
      dims.className = 'material-card-dims';
      dims.textContent = `${mat.width} × ${mat.height}`;
      card.appendChild(dims);

      grid.appendChild(card);
      viewCanvases.push({ view: canvas, src: mat.canvas });
    }
  }

  function open() {
    if (isOpen) return;
    isOpen = true;
    buildGrid();
    backdrop.hidden = false;
    rafId = requestAnimationFrame(syncFrame);
  }

  function close() {
    if (!isOpen) return;
    isOpen = false;
    backdrop.hidden = true;
    if (rafId !== null) {
      cancelAnimationFrame(rafId);
      rafId = null;
    }
    if (typeof onClose === 'function') onClose();
  }

  function dispose() {
    close();
    if (backdrop.parentNode) backdrop.parentNode.removeChild(backdrop);
  }

  closeBtn.addEventListener('click', close);
  backdrop.addEventListener('click', (e) => {
    if (e.target === backdrop) close();
  });
  keyHandler = (e) => {
    if (e.key === 'Escape' && isOpen) close();
  };
  window.addEventListener('keydown', keyHandler);

  return { open, close, dispose };
}
