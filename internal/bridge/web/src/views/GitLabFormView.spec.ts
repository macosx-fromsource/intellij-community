// @vitest-environment jsdom
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import { api } from '@/lib/api/client'
import GitLabFormView from './GitLabFormView.vue'

vi.mock('@/lib/api/client', () => ({
  api: {
    GET: vi.fn<() => Promise<unknown>>(),
    POST: vi.fn<() => Promise<unknown>>(),
    PUT: vi.fn<() => Promise<unknown>>(),
    DELETE: vi.fn<() => Promise<unknown>>(),
  },
  errorMessage: (error: unknown) => (error as { detail?: string })?.detail ?? 'error',
}))

// The values editor is a CodeMirror instance, which jsdom cannot lay out. The
// section logic does not depend on it, so it is stubbed down to a plain
// textarea that still round-trips through v-model, so a test can drive it.
vi.mock('@/components/YamlEditor.vue', () => ({
  default: {
    name: 'YamlEditor',
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<textarea :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))

const mockApi = api as unknown as { GET: Mock; POST: Mock }

/** Mounts the form and lets its mounted hook settle. */
async function mountForm(): Promise<VueWrapper> {
  const wrapper = mount(GitLabFormView)
  await flushPromises()

  return wrapper
}

const push = vi.fn<(to: unknown) => void>()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

/** Titles of the sidebar entries, in the order the sidebar shows them. */
const sectionTitles = [
  'Basics',
  'Dependencies',
  'AI Gateway',
  'Artifact Registry',
  'Orbit',
  'Secret Manager',
  'Siphon',
  'Overrides',
]

/** The sidebar button whose text starts with the given title. */
function navButton(wrapper: VueWrapper, title: string) {
  return wrapper.findAll('.sidebar button').find((candidate) => candidate.text().startsWith(title))!
}

/** Strips the trailing "On"/"Soon" status badge some sidebar entries carry. */
function stripStatus(text: string): string {
  return text.replace(/(On|Soon)$/, '').trim()
}

/** Returns the title of the section the sidebar marks as current. */
function currentSection(wrapper: VueWrapper): string {
  return stripStatus(wrapper.get('[aria-current="page"]').text())
}

/** Selects a section from the sidebar. */
async function selectSection(wrapper: VueWrapper, title: string) {
  const button = wrapper
    .findAll('.sidebar button')
    .find((candidate) => candidate.text().includes(title))

  await button!.trigger('click')
}

/**
 * The panel of the section that is shown. The others stay in the DOM behind
 * `v-show`, so a query has to be scoped to this one to reach the right field.
 */
function panel(wrapper: VueWrapper) {
  return wrapper
    .findAll('.section-panel')
    .find((candidate) => (candidate.element as HTMLElement).style.display !== 'none')!
}

/** The fieldset of the named group within the visible section. */
function group(wrapper: VueWrapper, legend: string) {
  return panel(wrapper)
    .findAll('fieldset')
    .find((candidate) => candidate.get('legend').text().includes(legend))!
}

/** Fills an input by the label it sits under, within the visible section. */
async function fill(wrapper: VueWrapper, label: string, value: string) {
  const field = panel(wrapper)
    .findAll('label')
    .find((candidate) => candidate.text().startsWith(label))

  await field!.get('input').setValue(value)
}

describe('GitLabFormView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('starts on Basics and lists every section, including add-ons', async () => {
    const wrapper = await mountForm()

    expect(wrapper.findAll('.sidebar button').map((button) => stripStatus(button.text()))).toEqual(
      sectionTitles,
    )
    expect(currentSection(wrapper)).toBe('Basics')
  })

  it('disables the not-yet-configurable add-ons', async () => {
    const wrapper = await mountForm()

    for (const title of ['Siphon', 'Orbit', 'Artifact Registry', 'AI Gateway']) {
      const button = wrapper.findAll('.sidebar button').find((candidate) => candidate.text().startsWith(title))!

      expect(button.attributes('disabled')).toBeDefined()

      await button.trigger('click')
      expect(currentSection(wrapper)).toBe('Basics')
    }
  })

  it('jumps straight to any section without validating the ones skipped', async () => {
    const wrapper = await mountForm()

    await selectSection(wrapper, 'Overrides')

    expect(currentSection(wrapper)).toBe('Overrides')
    expect(wrapper.text()).not.toContain('A name is required.')
  })

  // The name identifies the resource, so submitting with it empty holds the
  // form on the section it belongs to, wherever the form currently is.
  it('sends an incomplete submit back to the section with the problem', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Name', '')
    await selectSection(wrapper, 'Overrides')
    await wrapper.get('form').trigger('submit')

    expect(currentSection(wrapper)).toBe('Basics')
    expect(wrapper.text()).toContain('A name is required.')
    expect(mockApi.POST).not.toHaveBeenCalled()
  })

  // The namespace is the one the Operator installation owns, and the form offers
  // no choice of it.
  it('creates in gitlab-system without asking', async () => {
    const wrapper = await mountForm()

    const labels = panel(wrapper)
      .findAll('label span:first-child')
      .map((label) => label.text())

    expect(labels).not.toContain('Namespace')
    expect(panel(wrapper).text()).toContain('Created in the gitlab-system namespace.')
  })

  // Nothing about which versions the Operator can render is known here, so the
  // field starts empty rather than guessing.
  it('leaves the chart version empty by default', async () => {
    const wrapper = await mountForm()

    const input = panel(wrapper).get('input[name="chart-version"]')

    expect((input.element as HTMLInputElement).value).toBe('')
    expect(mockApi.GET).not.toHaveBeenCalled()
  })

  // The field is free-form: what renders is whatever the Operator bundles or
  // can pull, which the SPA has no way to know.
  it('takes any version', async () => {
    mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })

    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '11.0.0-rc1')
    await wrapper.get('form').trigger('submit')

    expect(mockApi.POST).toHaveBeenCalled()
  })

  // Nothing renders without a version, so the field cannot be cleared.
  it('requires a version', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '  ')
    await wrapper.get('form').trigger('submit')

    expect(currentSection(wrapper)).toBe('Basics')
    expect(wrapper.text()).toContain('A chart version is required.')
    expect(mockApi.POST).not.toHaveBeenCalled()
  })

  // A data store connection is all three fields or none, and the check belongs
  // to the section that holds them, which submitting jumps back to.
  it('reports an incomplete dependency on the section that owns it', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await selectSection(wrapper, 'Dependencies')
    await fill(wrapper, 'Hostname', 'gitlab-postgresql')
    await selectSection(wrapper, 'Overrides')
    await wrapper.get('form').trigger('submit')

    expect(currentSection(wrapper)).toBe('Dependencies')
    expect(wrapper.text()).toContain('PostgreSQL needs a hostname')
  })

  // Object storage is one Secret rather than a host and a Secret, and it is
  // still all or nothing.
  it('reports an incomplete object storage connection', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await selectSection(wrapper, 'Dependencies')
    await group(wrapper, 'Object storage').findAll('input')[0]!.setValue('gitlab-object-storage')
    await wrapper.get('form').trigger('submit')

    expect(currentSection(wrapper)).toBe('Dependencies')
    expect(wrapper.text()).toContain('Object storage needs both a Secret name and a Secret key.')
  })

  it('marks a section with an error in the sidebar', async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Name', '')
    await wrapper.get('form').trigger('submit')

    expect(navButton(wrapper, 'Basics').find('.nav-item-badge').exists()).toBe(true)
  })

  it('marks a section as having unsaved changes once it is edited, and clears it on save', async () => {
    mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })

    const wrapper = await mountForm()

    expect(navButton(wrapper, 'Basics').find('.nav-item-dot').exists()).toBe(false)

    await fill(wrapper, 'Chart version', '10.2.2')
    expect(navButton(wrapper, 'Basics').find('.nav-item-dot').exists()).toBe(true)

    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(navButton(wrapper, 'Basics').find('.nav-item-dot').exists()).toBe(false)
  })

  // Unlike Siphon, Secret Manager's fields really are sent (folded into
  // `chart.values`), so a successful save clears its mark like any other tab.
  it('marks Secret Manager unsaved once enabled, and clears it once saved', async () => {
    mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })

    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await selectSection(wrapper, 'Secret Manager')
    await panel(wrapper).get('input[type="checkbox"]').setValue(true)

    const fields = group(wrapper, 'PostgreSQL').findAll('input')
    await fields[0]!.setValue('openbao-postgresql')
    await fields[4]!.setValue('openbao-db-password')
    await fields[5]!.setValue('password')
    await group(wrapper, 'ServiceAccount').get('input').setValue('openbao')

    expect(navButton(wrapper, 'Secret Manager').find('.nav-item-dot').exists()).toBe(true)

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(navButton(wrapper, 'Secret Manager').find('.nav-item-dot').exists()).toBe(false)
  })

  it('unlocks the Secret Manager PostgreSQL and ServiceAccount fields on enable', async () => {
    const wrapper = await mountForm()

    await selectSection(wrapper, 'Secret Manager')
    expect(group(wrapper, 'PostgreSQL').attributes('disabled')).toBeDefined()
    expect(group(wrapper, 'ServiceAccount').attributes('disabled')).toBeDefined()

    await panel(wrapper).get('input[type="checkbox"]').setValue(true)
    expect(group(wrapper, 'PostgreSQL').attributes('disabled')).toBeUndefined()
    expect(group(wrapper, 'ServiceAccount').attributes('disabled')).toBeUndefined()
  })

  it("requires Secret Manager's database hostname and password Secret once enabled", async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await selectSection(wrapper, 'Secret Manager')
    await panel(wrapper).get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')

    expect(currentSection(wrapper)).toBe('Secret Manager')
    expect(wrapper.text()).toContain("Secret Manager needs its PostgreSQL database's hostname")
    expect(mockApi.POST).not.toHaveBeenCalled()
  })

  // The CRD's own bounds on the port (1-65535) are checked client-side too,
  // so an out-of-range value is caught here rather than reaching the API as
  // a raw Kubernetes validation error.
  it("rejects a Secret Manager PostgreSQL port outside 1-65535", async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await selectSection(wrapper, 'Secret Manager')
    await panel(wrapper).get('input[type="checkbox"]').setValue(true)

    const fields = group(wrapper, 'PostgreSQL').findAll('input')
    await fields[0]!.setValue('openbao-postgresql')
    await fields[1]!.setValue('99999999')
    await fields[4]!.setValue('openbao-db-password')
    await fields[5]!.setValue('password')

    await wrapper.get('form').trigger('submit')

    expect(currentSection(wrapper)).toBe('Secret Manager')
    expect(wrapper.text()).toContain('The PostgreSQL port must be a number between 1 and 65535.')
    expect(mockApi.POST).not.toHaveBeenCalled()
  })

  // OpenBao needs RBAC on Pods that the bridge and the Operator do not
  // create, so the ServiceAccount name is required once Secret Manager is
  // enabled, and reported alongside an incomplete PostgreSQL connection.
  it("requires Secret Manager's ServiceAccount name once enabled", async () => {
    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await selectSection(wrapper, 'Secret Manager')
    await panel(wrapper).get('input[type="checkbox"]').setValue(true)

    const fields = group(wrapper, 'PostgreSQL').findAll('input')
    await fields[0]!.setValue('openbao-postgresql')
    await fields[4]!.setValue('openbao-db-password')
    await fields[5]!.setValue('password')

    await wrapper.get('form').trigger('submit')

    expect(currentSection(wrapper)).toBe('Secret Manager')
    expect(wrapper.text()).toContain('Secret Manager needs the name of a ServiceAccount')
    expect(mockApi.POST).not.toHaveBeenCalled()
  })

  // Secret Manager is a structured field of the resource, like PostgreSQL or Redis,
  // not something folded into chart.values.
  it('sends the enabled Secret Manager configuration as its own resource field', async () => {
    mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })

    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')

    await selectSection(wrapper, 'Secret Manager')
    await panel(wrapper).get('input[type="checkbox"]').setValue(true)

    const fields = group(wrapper, 'PostgreSQL').findAll('input')
    await fields[0]!.setValue('openbao-postgresql')
    await fields[1]!.setValue('5433')
    await fields[2]!.setValue('openbao_db')
    await fields[3]!.setValue('openbao_user')
    await fields[4]!.setValue('openbao-db-password')
    await fields[5]!.setValue('password')
    await group(wrapper, 'ServiceAccount').get('input').setValue('openbao')

    await wrapper.get('form').trigger('submit')

    const body = mockApi.POST.mock.calls[0]![1].body
    expect(body.openbao).toEqual({
      postgresql: {
        host: 'openbao-postgresql',
        port: 5433,
        database: 'openbao_db',
        username: 'openbao_user',
        passwordSecretRef: { name: 'openbao-db-password', key: 'password' },
      },
      serviceAccount: { name: 'openbao' },
    })
    expect(body.chart.values).toEqual({})
  })

  it('leaves the Secret Manager field out of the request when the add-on is off', async () => {
    mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })

    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await wrapper.get('form').trigger('submit')

    const body = mockApi.POST.mock.calls[0]![1].body
    expect(body.openbao).toBeUndefined()
  })

  it('creates the resource from every section on submit', async () => {
    mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })

    const wrapper = await mountForm()

    await fill(wrapper, 'Chart version', '10.2.2')
    await fill(wrapper, 'Hostname', 'gitlab.example.com')
    await selectSection(wrapper, 'Dependencies')

    const psql = group(wrapper, 'PostgreSQL').findAll('input')
    await psql[0]!.setValue('gitlab-postgresql')
    await psql[1]!.setValue('gitlab-postgresql-password')
    await psql[2]!.setValue('password')

    const valkey = group(wrapper, 'Valkey').findAll('input')
    await valkey[0]!.setValue('gitlab-valkey')
    await valkey[1]!.setValue('gitlab-valkey-auth')
    await valkey[2]!.setValue('default')

    const storage = group(wrapper, 'Object storage').findAll('input')
    await storage[0]!.setValue('gitlab-object-storage')
    await storage[1]!.setValue('connection')

    await wrapper.get('form').trigger('submit')

    expect(mockApi.POST).toHaveBeenCalledWith('/api/v1/namespaces/{namespace}/gitlabs', {
      params: { path: { namespace: 'gitlab-system' } },
      body: {
        name: 'gitlab',
        namespace: 'gitlab-system',
        hostname: 'gitlab.example.com',
        edition: 'ee',
        license: undefined,
        postgresql: {
          host: 'gitlab-postgresql',
          passwordSecretRef: { name: 'gitlab-postgresql-password', key: 'password' },
        },
        redis: {
          host: 'gitlab-valkey',
          passwordSecretRef: { name: 'gitlab-valkey-auth', key: 'default' },
        },
        objectStorage: {
          connectionSecretRef: { name: 'gitlab-object-storage', key: 'connection' },
        },
        chart: { version: '10.2.2', values: {} },
      },
    })
  })

})
