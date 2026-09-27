import type { Directive } from "vue";

// registers the element with the given IntersectionObserver for as long as it is mounted
export default {
  mounted(el, binding) {
    binding.value?.observe(el);
  },
  updated(el, binding) {
    if (binding.value === binding.oldValue) return;
    binding.oldValue?.unobserve(el);
    binding.value?.observe(el);
  },
  beforeUnmount(el, binding) {
    binding.value?.unobserve(el);
  },
} satisfies Directive<HTMLElement, IntersectionObserver | null>;
