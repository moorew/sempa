<script lang="ts">
  // Settings primitives (grouped surfaces, rows, pill switch, filled inputs),
  // shared by every page under /settings. No wrapper element: pages keep their
  // own layout.
  import './settings.css';
  let { children } = $props();
</script>

{@render children()}
