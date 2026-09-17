import { computed, ref, type ComputedRef } from 'vue'

/**
 * Tracks whether a form section's fields differ from their last loaded or
 * saved state, for the "unsaved changes" mark next to it in the sidebar.
 *
 * `snapshot` must return a JSON-serializable, deterministic value built from
 * the section's own reactive fields. Every field a section owns is a string,
 * a boolean, or a small flat record of those, so comparing two JSON strings
 * is an exact and cheap stand-in for a deep-equality check.
 */
export function useDirtyTracking(snapshot: () => unknown): {
  isDirty: ComputedRef<boolean>
  resetBaseline: () => void
} {
  const baseline = ref(JSON.stringify(snapshot()))

  const isDirty = computed(() => JSON.stringify(snapshot()) !== baseline.value)

  function resetBaseline() {
    baseline.value = JSON.stringify(snapshot())
  }

  return { isDirty, resetBaseline }
}
