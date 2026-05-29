let globalUIListenersBound = false;

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

export class UIListSearch {
  constructor(input) {
    this.input = input;
    this.root = input.closest('[data-ui-list-search]') || input.parentElement;
    this.items = Array.from(this.root?.querySelectorAll('[data-ui-list-search-item]') || []);
    this.emptyState = this.root?.querySelector('[data-ui-list-search-empty]');

    if (this.input.dataset.uiListSearchInitialized === 'true') return;
    this.input.dataset.uiListSearchInitialized = 'true';

    this.input.addEventListener('input', () => this.filter());
    this.input.addEventListener('search', () => this.filter());
    this.input.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') event.preventDefault();
    });
    this.filter();
  }

  filter() {
    const query = this.input.value.trim().toLowerCase();
    let visibleCount = 0;

    this.items.forEach((item) => {
      const searchableText = (item.dataset.searchText || item.textContent || '').toLowerCase();
      const matches = query === '' || searchableText.includes(query);
      item.classList.toggle('hidden', !matches);
      if (matches) visibleCount += 1;
    });

    if (this.emptyState) {
      this.emptyState.classList.toggle('hidden', visibleCount !== 0 || query === '');
    }
  }
}

export function initUIComponents(root = document) {
  const dropdowns = [];
  root.querySelectorAll('[data-ui-dropdown]').forEach((node) => {
    const dropdown = node.matches('[data-ui-multiselect]') ? new UIMultiSelect(node) : new UIDropdown(node);
    node.__uiDropdownInstance = dropdown;
    dropdowns.push(dropdown);
  });

  const modals = Array.from(root.querySelectorAll('[data-ui-modal]')).map((node) => {
    const modal = new UIModal(node);
    node.__uiModalInstance = modal;
    return modal;
  });
  root.querySelectorAll('[data-ui-list-search-input]').forEach((node) => new UIListSearch(node));

  if (!globalUIListenersBound) {
    globalUIListenersBound = true;

    document.addEventListener('click', (event) => {
      document.querySelectorAll('[data-ui-dropdown]').forEach((node) => {
        const dropdown = node.__uiDropdownInstance;
        if (!dropdown) return;
        if (!dropdown.root.contains(event.target)) dropdown.close();
      });
    });

    document.addEventListener('keydown', (event) => {
      if (event.key !== 'Escape') return;
      document.querySelectorAll('[data-ui-dropdown]').forEach((node) => node.__uiDropdownInstance?.close());
      document.querySelectorAll('[data-ui-modal]').forEach((node) => node.__uiModalInstance?.close());
    });
  }

  return { dropdowns, modals };
}
