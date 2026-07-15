<script setup lang="ts">
import { onBeforeUnmount, onMounted, useTemplateRef, watch } from 'vue'
import { EditorView, basicSetup } from 'codemirror'
import { Compartment, EditorState } from '@codemirror/state'
import { yaml } from '@codemirror/lang-yaml'
import { oneDark } from '@codemirror/theme-one-dark'

const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const host = useTemplateRef<HTMLDivElement>('host')
let view: EditorView | undefined

// Fixed-size editor so an empty document still renders a full box rather than a
// single line.
const sizeTheme = EditorView.theme({
  '&': { height: '260px', fontSize: '13px' },
  '.cm-scroller': { fontFamily: 'var(--font-mono, ui-monospace, monospace)' },
})

// Swap the color theme with the OS/browser color scheme: one-dark in dark mode,
// CodeMirror's light default otherwise.
const darkQuery = window.matchMedia('(prefers-color-scheme: dark)')
const themeCompartment = new Compartment()
const colorTheme = () => (darkQuery.matches ? oneDark : [])

function applyColorTheme() {
  view?.dispatch({ effects: themeCompartment.reconfigure(colorTheme()) })
}

onMounted(() => {
  if (!host.value) {
    return
  }

  view = new EditorView({
    parent: host.value,
    state: EditorState.create({
      doc: props.modelValue,
      extensions: [
        basicSetup,
        yaml(),
        sizeTheme,
        themeCompartment.of(colorTheme()),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            emit('update:modelValue', update.state.doc.toString())
          }
        }),
      ],
    }),
  })

  darkQuery.addEventListener('change', applyColorTheme)
})

// Reflect external changes (e.g. loading an existing resource) into the editor.
watch(
  () => props.modelValue,
  (value) => {
    if (view && value !== view.state.doc.toString()) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
    }
  },
)

onBeforeUnmount(() => {
  darkQuery.removeEventListener('change', applyColorTheme)
  view?.destroy()
})
</script>

<template>
  <div ref="host" class="yaml-editor"></div>
</template>

<style scoped>
.yaml-editor {
  border: 1px solid var(--color-border);
  border-radius: 4px;
  overflow: hidden;
}

.yaml-editor :deep(.cm-editor.cm-focused) {
  outline: none;
}
</style>
