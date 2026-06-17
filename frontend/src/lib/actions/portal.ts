// Move a node to a target element (default <body>) for its lifetime, then put
// nothing back — used to lift modals out of the page wrapper so a transformed
// ancestor (the page-in entrance) can't become their containing block, and so
// they always paint above the app shell. Browser-only (actions never run on SSR).

export function portal(node: HTMLElement, target: HTMLElement | string = document.body) {
  let targetEl: HTMLElement | null = null;

  function mount(t: HTMLElement | string) {
    targetEl = typeof t === 'string' ? document.querySelector<HTMLElement>(t) : t;
    if (targetEl) targetEl.appendChild(node);
  }

  mount(target);

  return {
    update(t: HTMLElement | string) { mount(t); },
    destroy() { node.parentNode?.removeChild(node); },
  };
}
