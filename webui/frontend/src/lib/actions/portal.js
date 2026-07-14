/**
 * Svelte action: moves the node to document.body on mount, removes it on
 * destroy. Used by components that render a positioned overlay (tooltip,
 * dropdown menu) which needs to escape overflow:hidden / scroll container
 * clipping from its logical parent.
 */
export function portal(node) {
  document.body.appendChild(node);
  return {
    destroy() {
      if (node.parentNode) node.parentNode.removeChild(node);
    },
  };
}
