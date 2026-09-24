<script setup lang="ts">
import { computed, onMounted, ref, useTemplateRef } from 'vue'
import { useRouter } from 'vue-router'

import '@/assets/gitlab-form.css'
import BasicsSection from '@/components/gitlab-form/BasicsSection.vue'
import DependenciesSection from '@/components/gitlab-form/DependenciesSection.vue'
import FormSidebar, { type NavGroup } from '@/components/gitlab-form/FormSidebar.vue'
import OpenBaoSection from '@/components/gitlab-form/OpenBaoSection.vue'
import OverridesSection from '@/components/gitlab-form/OverridesSection.vue'
import SiphonSection from '@/components/gitlab-form/SiphonSection.vue'
import type { GitLabResource } from '@/lib/api/client'
import { useGitLabsStore } from '@/stores/gitlabs'
import { useSiphonsStore } from '@/stores/siphons'

const props = defineProps<{
  namespace?: string
  name?: string
}>()

const store = useGitLabsStore()
const siphons = useSiphonsStore()
const router = useRouter()

const isEdit = Boolean(props.namespace && props.name)

/**
 * Whether the instance exists, which is not the same as having opened the
 * edit page: a create whose Siphon then fails leaves the instance stored and
 * the form open on it. From that point the form edits what it just created —
 * it saves with `PUT`, and the name is fixed — so a second submit updates the
 * instance rather than creating it again under the same name, which the API
 * server rejects.
 */
const created = ref(isEdit)

/**
 * What the last submit did, when the form stays open afterwards. The errors
 * of the stores report what failed; this reports what nonetheless happened,
 * which is otherwise invisible.
 */
const notice = ref<string | null>(null)

/**
 * Where a new instance is created. The Operator installation owns this
 * namespace, so the form does not offer a choice; an existing resource is
 * edited wherever it already lives.
 */
const defaultNamespace = 'gitlab-system'

const namespace = ref(props.namespace ?? defaultNamespace)

/** A section of the form, addressed by the sidebar. */
type SectionId = 'basics' | 'dependencies' | 'overrides' | 'openbao' | 'siphon'

const activeSection = ref<SectionId>('basics')

/**
 * The subtitle shown above each section's fields. The add-ons have none:
 * their description already lives beneath their own checkbox, so a subtitle
 * up here would just repeat it.
 */
const summaries: Record<SectionId, string> = {
  basics: 'What the instance is, and which chart deploys it.',
  dependencies: 'The data stores the instance connects to.',
  openbao: '',
  siphon: '',
  overrides: 'The chart values, for everything the sections above do not cover.',
}

const currentSummary = computed(() => summaries[activeSection.value])

const basics = useTemplateRef<InstanceType<typeof BasicsSection>>('basics')
const dependencies = useTemplateRef<InstanceType<typeof DependenciesSection>>('dependencies')
const openbao = useTemplateRef<InstanceType<typeof OpenBaoSection>>('openbao')
const siphon = useTemplateRef<InstanceType<typeof SiphonSection>>('siphon')
const overrides = useTemplateRef<InstanceType<typeof OverridesSection>>('overrides')

/**
 * Add-ons with no panel yet. They are listed so the sidebar shows the full
 * set of GitLab subcomponents the Operator is growing toward, but each is a
 * disabled placeholder until it has fields of its own to configure.
 */
const plannedAddons = [
  { id: 'orbit', title: 'Orbit' },
  { id: 'artifact-registry', title: 'Artifact Registry' },
  { id: 'ai-gateway', title: 'AI Gateway' },
] as const

/**
 * The sidebar, grouped as the core instance configuration (always present),
 * **Add-ons** (GitLab subcomponents an instance can optionally turn on), and
 * **Advanced** (settings most instances leave alone). Each item's
 * error/dirty/enabled state is read from the section component itself,
 * through the template refs above.
 */
const navGroups = computed<NavGroup[]>(() => [
  {
    items: [
      { id: 'basics', title: 'Basics', hasError: basics.value?.hasError, isDirty: basics.value?.isDirty },
      {
        id: 'dependencies',
        title: 'Dependencies',
        hasError: dependencies.value?.hasError,
        isDirty: dependencies.value?.isDirty,
      },
    ],
  },
  {
    label: 'Add-ons',
    items: [
      {
        id: 'openbao',
        title: 'Secret Manager',
        hasError: openbao.value?.hasError,
        isDirty: openbao.value?.isDirty,
        status: openbao.value?.enabled ? 'On' : undefined,
      },
      {
        id: 'siphon',
        title: 'Siphon',
        hasError: siphon.value?.hasError,
        isDirty: siphon.value?.isDirty,
        status: siphons.unreadable ? 'Unknown' : siphon.value?.enabled ? 'On' : undefined,
      },
      ...plannedAddons.map((entry) => ({ id: entry.id, title: entry.title, status: 'Soon', disabled: true })),
    ].sort((a, b) => a.title.localeCompare(b.title)),
  },
  {
    label: 'Advanced',
    items: [
      {
        id: 'overrides',
        title: 'Overrides',
        hasError: overrides.value?.hasError,
        isDirty: overrides.value?.isDirty,
      },
    ],
  },
])

/** What every section exposes to the parent, whatever its fields are. */
type Section = {
  validate: () => boolean
  resetBaseline: () => void
}

/**
 * The sections the GitLab resource itself is built from, which load from it
 * and are saved with it. Siphon is left out: it loads from its own resource
 * and is saved after the instance, so the parent names it where it differs.
 */
function instanceSections(): (Section & { loadFrom: (resource: GitLabResource) => void })[] {
  return [basics.value, dependencies.value, openbao.value, overrides.value].filter(
    (section) => section !== null,
  )
}

/** Every section, in the order a submit checks them. */
function allSections(): { id: SectionId; section: Section | null }[] {
  return [
    { id: 'basics', section: basics.value },
    { id: 'dependencies', section: dependencies.value },
    { id: 'openbao', section: openbao.value },
    { id: 'siphon', section: siphon.value },
    { id: 'overrides', section: overrides.value },
  ]
}

onMounted(async () => {
  if (isEdit) {
    const ok = await store.fetchOne(props.namespace!, props.name!)
    if (ok && store.current) {
      for (const section of instanceSections()) {
        section.loadFrom(store.current)
      }
    }

    // Siphon is a resource of its own, fetched separately; an instance
    // without one leaves the add-on off.
    if (await siphons.fetchFor(props.namespace!, props.name!)) {
      siphon.value?.loadFrom(siphons.current)
    }
  } else {
    // The stores outlive the page. A new instance fetches nothing, so
    // whatever the last one left behind would be taken for this one's — an
    // error already dealt with, or a Siphon this instance does not have and
    // turning the add-on off would offer to delete.
    store.reset()
    siphons.reset()
  }

  // Taken once the fetched values (or a new instance's defaults) have
  // settled, so editing what was just loaded is what "unsaved" means, not
  // loading it.
  for (const { section } of allSections()) {
    section?.resetBaseline()
  }
})

/**
 * What became of the Siphon. Declining the confirmation is not a failure —
 * nothing went wrong and there is no error to show — but it does leave a
 * resource the form no longer describes, so it is not a save either.
 */
type SiphonOutcome = 'saved' | 'failed' | 'cancelled'

/**
 * Saves the Siphon of the instance after the instance itself: creating or
 * replacing it while the add-on is on, and deleting the resource once it is
 * turned off. Deletion is confirmed, because it stops a running pipeline and
 * leaves the PostgreSQL and NATS objects behind.
 */
async function persistSiphon(name: string): Promise<SiphonOutcome> {
  // A Siphon that could not be read is left alone: the form knows nothing of
  // it to save or delete.
  if (siphons.unreadable) {
    return 'saved'
  }

  const resource = siphon.value!.toPartial()

  if (resource) {
    return (await siphons.save(namespace.value, name, resource)) ? 'saved' : 'failed'
  }

  if (!siphons.current) {
    return 'saved'
  }

  const confirmed = confirm(
    `Delete the Siphon "${siphons.current.name}"? The pipeline stops, and its PostgreSQL publication and replication slot and its NATS stream are left behind.`,
  )

  if (!confirmed) {
    return 'cancelled'
  }

  return (await siphons.remove(namespace.value, name)) ? 'saved' : 'failed'
}

async function submit() {
  notice.value = null

  for (const { id, section } of allSections()) {
    if (!section?.validate()) {
      activeSection.value = id

      return
    }
  }

  const basicsData = basics.value!.toPartial()

  const resource: GitLabResource = {
    name: basicsData.name,
    namespace: namespace.value,
    hostname: basicsData.hostname,
    edition: basicsData.edition,
    license: basicsData.license,
    ...dependencies.value!.toPartial(),
    openbao: openbao.value!.toPartial(),
    chart: {
      version: basicsData.version,
      values: overrides.value!.toPartial(),
    },
  }

  const ok = created.value
    ? await store.update(namespace.value, resource.name, resource)
    : await store.create(resource)

  if (!ok) {
    return
  }

  // Stored, whatever happens to the Siphon below: the sections it was built
  // from are saved, and a resubmit has to update this instance rather than
  // create a second one under the same name.
  created.value = true

  for (const section of instanceSections()) {
    section.resetBaseline()
  }

  // The Siphon references the instance, so it follows it rather than the
  // other way around, and only once the instance is stored.
  const outcome = await persistSiphon(resource.name)

  if (outcome !== 'saved') {
    notice.value =
      outcome === 'cancelled'
        ? 'The instance was saved, and its Siphon kept. Turn the add-on back on, or save again and confirm the deletion.'
        : 'The instance was saved, but its Siphon was not. Saving again retries it: the instance is updated, not created a second time.'
    activeSection.value = 'siphon'

    return
  }

  siphon.value!.resetBaseline()
  router.push({ name: 'gitlabs' })
}
</script>

<template>
  <section>
    <h1>{{ created ? 'Edit GitLab instance' : 'New GitLab instance' }}</h1>

    <p v-if="store.error" class="error" role="alert">{{ store.error }}</p>
    <p v-if="siphons.error" class="error" role="alert">{{ siphons.error }}</p>
    <p v-if="notice" class="notice" role="status">{{ notice }}</p>

    <form class="editor gitlab-form" @submit.prevent="submit">
      <FormSidebar :groups="navGroups" :active-id="activeSection" @select="activeSection = $event as SectionId" />

      <div class="content">
        <p v-if="currentSummary" class="hint section-summary">{{ currentSummary }}</p>

        <!--
          v-show rather than v-if: the values editor inside OverridesSection
          is a CodeMirror instance that mounts once, and every section keeps
          its state while another one is shown.
        -->
        <BasicsSection
          v-show="activeSection === 'basics'"
          ref="basics"
          :namespace="namespace"
          :is-edit="created"
          :initial-name="props.name ?? 'gitlab'"
        />

        <DependenciesSection v-show="activeSection === 'dependencies'" ref="dependencies" />

        <OverridesSection v-show="activeSection === 'overrides'" ref="overrides" />

        <OpenBaoSection v-show="activeSection === 'openbao'" ref="openbao" />

        <SiphonSection v-show="activeSection === 'siphon'" ref="siphon" :unreadable="siphons.unreadable" />

        <div class="actions">
          <button type="submit" :disabled="store.loading || siphons.loading">
            {{ created ? 'Save' : 'Create' }}
          </button>
          <button type="button" @click="router.push({ name: 'gitlabs' })">Cancel</button>
        </div>
      </div>
    </form>
  </section>
</template>

<style scoped>
.editor {
  display: flex;
  align-items: flex-start;
  gap: 2rem;
  margin-top: 1rem;
}

.content {
  flex: 1 1 auto;
  min-width: 0;
  max-width: 640px;
}

.section-summary {
  margin: 0 0 1rem;
}

.actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.error {
  color: #d64541;
  white-space: pre-wrap;
}

.notice {
  color: #8f6200;
  white-space: pre-wrap;
}
</style>
