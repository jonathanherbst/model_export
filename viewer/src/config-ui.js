function uint32ToCss(color) {
  let a = (color >>> 24) & 0xff;
  const r = (color >>> 16) & 0xff;
  const g = (color >>> 8) & 0xff;
  const b = color & 0xff;
  if(a == 0) {
    a = 255
  }
  return `rgba(${r},${g},${b},${(a / 255).toFixed(2)})`;
}

export function setupConfigPanel(configurations, onChange, randomizeExclusions = new Set()) {
  const configPanel = document.createElement('div');
  configPanel.id = 'config-panel';
  configPanel.className = 'collapsible';

  const header = document.createElement('div');
  header.className = 'config-header';
  header.innerHTML =
    '<span class="chevron">&#9660;</span> Configuration';

  const panelBody = document.createElement('div');
  panelBody.className = 'config-body';

  const randomizeBtn = document.createElement('button');
  randomizeBtn.type = 'button';
  randomizeBtn.textContent = 'Randomize';
  randomizeBtn.addEventListener('click', () => {
    randomizeChoices();
  });
  panelBody.appendChild(randomizeBtn);

  const defaultValueIds = new Set();
  for (const [, choices] of configurations) {
    if (choices.length > 0) {
      defaultValueIds.add(choices[0].id);
    }
  }

  let optionIdx = 0;
  for (const [optionName, choices] of configurations) {
    const groupEl = document.createElement('div');
    groupEl.className = 'config-option-group';
    if (optionIdx === 0) groupEl.classList.add('first');

    const label = document.createElement('div');
    label.className = 'config-option-label';
    label.textContent = optionName;
    groupEl.appendChild(label);

    const choicesRow = document.createElement('div');
    choicesRow.className = 'config-choices-row';

    choices.forEach((choice, choiceIdx) => {
      const item = document.createElement('div');
      item.className = 'config-choice';

      if (
        choice.color !== undefined &&
        choice.color !== null &&
        choice.color !== 0
      ) {
        const swatch = document.createElement('span');
        swatch.className = 'config-swatch';
        swatch.style.backgroundColor = uint32ToCss(choice.color);
        item.appendChild(swatch);
      }

      if (choice.name && choice.name.trim()) {
        const nameSpan = document.createElement('span');
        nameSpan.className = 'config-choice-name';
        nameSpan.textContent = choice.name;
        item.appendChild(nameSpan);
      }

      if (
        (!choice.name || !choice.name.trim()) &&
        (!choice.color || choice.color === 0)
      ) {
        const nameSpan = document.createElement('span');
        nameSpan.className = 'config-choice-name';
        nameSpan.textContent = `${optionName} ${choiceIdx + 1}`;
        item.appendChild(nameSpan);
      }

      if (defaultValueIds.has(choice.id)) {
        item.classList.add('selected');
      }

      item.dataset.id = choice.id;
      item.dataset.option = optionName;
      item.addEventListener('click', () => {
        const siblings =
          groupEl.querySelectorAll('.config-choice');
        siblings.forEach((s) => s.classList.remove('selected'));
        item.classList.add('selected');
        onChange();
      });

      choicesRow.appendChild(item);
    })

    groupEl.appendChild(choicesRow);
    panelBody.appendChild(groupEl);
    optionIdx++;
  }

  header.addEventListener('click', () => {
    panelBody.hidden = !panelBody.hidden;
    header.querySelector(
      '.chevron'
    ).innerHTML = panelBody.hidden ? '&#9654;' : '&#9660;';
  });

  configPanel.appendChild(header);
  configPanel.appendChild(panelBody);
  document.getElementById('ui-panel').appendChild(configPanel);

  function randomizeChoices() {
    const groups =
      panelBody.querySelectorAll('.config-option-group');
    groups.forEach((group) => {
      const firstChoice = group.querySelector('.config-choice');
      const optionName = firstChoice?.dataset.option;
      if (optionName && randomizeExclusions.has(optionName))
        return;
      const choices = group.querySelectorAll('.config-choice');
      if (choices.length <= 1) return;
      const randomIdx = Math.floor(Math.random() * choices.length);
      choices.forEach((c) => c.classList.remove('selected'));
      choices[randomIdx].classList.add('selected');
    });
    onChange();
  }

  return {
    getCurrentChoices() {
      const ids = [];
      const groups =
        panelBody.querySelectorAll('.config-option-group');
      groups.forEach((group) => {
        const selected = group.querySelector(
          '.config-choice.selected'
        );
        if (selected) {
          ids.push(parseInt(selected.dataset.id, 10));
        }
      });
      return ids;
    },

    setDefaultChoices() {
      const items =
        panelBody.querySelectorAll('.config-choice');
      items.forEach((item) => {
        item.classList.remove('selected');
        if (
          defaultValueIds.has(parseInt(item.dataset.id, 10))
        ) {
          item.classList.add('selected');
        }
      });
    },
  };
}
