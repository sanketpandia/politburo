export class UIDropdown {
  constructor(root) {
    this.root = root;
    this.trigger = root.querySelector('[data-ui-dropdown-trigger]');
    this.panel = root.querySelector('[data-ui-dropdown-panel]');

    this.trigger?.addEventListener('click', () => this.toggle());
    this.root.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') this.close();
    });
  }

  open() {
    this.root.classList.add('open');
    this.trigger?.setAttribute('aria-expanded', 'true');
  }

  close() {
    this.root.classList.remove('open');
    this.trigger?.setAttribute('aria-expanded', 'false');
  }

  toggle() {
    this.root.classList.contains('open') ? this.close() : this.open();
  }
}

export class UIMultiSelect extends UIDropdown {
  constructor(root) {
    super(root);
    this.defaultLabel = root.dataset.placeholder || 'Choose options';
    this.valueLabel = root.querySelector('[data-ui-multiselect-label]');
    this.inputs = Array.from(root.querySelectorAll('input[type="checkbox"]'));

    this.inputs.forEach((input) => input.addEventListener('change', () => this.updateLabel()));
    this.updateLabel();
  }

  updateLabel() {
    if (!this.valueLabel) return;
    const selected = this.inputs.filter((input) => input.checked);
    if (selected.length === 0) {
      this.valueLabel.textContent = this.defaultLabel;
      return;
    }
    if (selected.length === 1) {
      const option = selected[0].closest('[data-ui-option]');
      this.valueLabel.textContent = option?.querySelector('[data-ui-option-name]')?.textContent?.trim() || '1 selected';
      return;
    }
    this.valueLabel.textContent = `${selected.length} selected`;
  }
}

export class UIModal {
  constructor(root) {
    this.root = root;
    this.closeButtons = Array.from(root.querySelectorAll('[data-ui-modal-close]'));

    this.closeButtons.forEach((button) => button.addEventListener('click', () => this.close()));
    this.root.addEventListener('click', (event) => {
      if (event.target === this.root) this.close();
    });
  }

  open() {
    this.root.classList.add('open');
    this.root.setAttribute('aria-hidden', 'false');
  }

  close() {
    this.root.classList.remove('open');
    this.root.setAttribute('aria-hidden', 'true');
  }
}

export function initUIComponents(root = document) {
  const dropdowns = [];
  root.querySelectorAll('[data-ui-dropdown]').forEach((node) => {
    dropdowns.push(node.matches('[data-ui-multiselect]') ? new UIMultiSelect(node) : new UIDropdown(node));
  });

  const modals = Array.from(root.querySelectorAll('[data-ui-modal]')).map((node) => new UIModal(node));

  document.addEventListener('click', (event) => {
    dropdowns.forEach((dropdown) => {
      if (!dropdown.root.contains(event.target)) dropdown.close();
    });
  });

  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape') return;
    dropdowns.forEach((dropdown) => dropdown.close());
    modals.forEach((modal) => modal.close());
  });

  return { dropdowns, modals };
}
