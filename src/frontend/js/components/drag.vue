<template>
  <div class="drag" :class="{ 'is-active': active }"></div>
</template>

<script lang="ts">
import { defineComponent } from "vue";

export default defineComponent({
  emits: ["resize", "reset"],
  data() {
    return { active: false };
  },
  mounted() {
    const el = this.$el as HTMLElement;
    let startX = 0;
    let initW = 0;
    const onPointerMove = (e: PointerEvent) => {
      this.$emit("resize", Math.round(initW + e.clientX - startX));
    };
    const onPointerUp = (e: PointerEvent) => {
      this.active = false;
      document.body.classList.remove("is-resizing");
      el.releasePointerCapture(e.pointerId);
      el.removeEventListener("pointermove", onPointerMove);
      el.removeEventListener("pointerup", onPointerUp);
      el.removeEventListener("pointercancel", onPointerUp);
    };
    el.addEventListener("pointerdown", (e: PointerEvent) => {
      if (e.button !== 0) return;
      e.preventDefault();
      startX = e.clientX;
      // measure the rendered column rather than relying on the stored width
      initW = el.parentElement!.getBoundingClientRect().width;
      this.active = true;
      document.body.classList.add("is-resizing");
      el.setPointerCapture(e.pointerId);
      el.addEventListener("pointermove", onPointerMove);
      el.addEventListener("pointerup", onPointerUp);
      el.addEventListener("pointercancel", onPointerUp);
    });
    el.addEventListener("dblclick", () => this.$emit("reset"));
  },
});
</script>
