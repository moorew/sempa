import { api } from '$lib/api';
import type { TagDefinition } from '$lib/types';

function createTagStore() {
  let definitions = $state<TagDefinition[]>([]);
  let loaded = false;

  async function load() {
    if (loaded) return;
    try {
      definitions = await api.tags.list();
      loaded = true;
    } catch {
      // non-fatal — tags just won't have colours until loaded
    }
  }

  function colorFor(name: string): string {
    const d = definitions.find(t => t.name.toLowerCase() === name.toLowerCase());
    return d?.color ?? '#6b7280';
  }

  function add(tag: TagDefinition) {
    const idx = definitions.findIndex(t => t.id === tag.id);
    if (idx >= 0) definitions[idx] = tag;
    else definitions = [...definitions, tag];
  }

  function remove(id: string) {
    definitions = definitions.filter(t => t.id !== id);
  }

  return {
    get definitions() { return definitions; },
    load,
    colorFor,
    add,
    remove,
  };
}

export const tagStore = createTagStore();
