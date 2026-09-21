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

const mockApi = api as unknown as { GET: Mock; POST: Mock; PUT: Mock; DELETE: Mock }

/** Mounts the form and lets its mounted hook settle. */
async function mountForm(props?: { namespace: string; name: string }): Promise<VueWrapper> {
  const wrapper = mount(GitLabFormView, props ? { props } : undefined)
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

    for (const title of ['Orbit', 'Artifact Registry', 'AI Gateway']) {
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

  // Siphon is a custom resource of its own, linked to the instance by its
  // reference, so the form drives a second endpoint rather than a field.
  describe('Siphon', () => {
    /** Fills the groups a Siphon needs, from the Siphon section. */
    async function fillSiphon(wrapper: VueWrapper) {
      await group(wrapper, 'Chart').get('input').setValue('1.21.0')

      const source = group(wrapper, 'PostgreSQL source').findAll('input')
      await source[0]!.setValue('gitlab-postgresql-rw.databases.svc.cluster.local')
      await source[4]!.setValue('gitlab-siphon-postgresql')
      await source[5]!.setValue('password')

      await group(wrapper, 'NATS queue').findAll('input')[0]!.setValue('nats://nats.nats.svc.cluster.local:4222')

      const sink = group(wrapper, 'ClickHouse sink').findAll('input')
      await sink[0]!.setValue('clickhouse.databases.svc.cluster.local')
      await sink[2]!.setValue('gitlab_clickhouse_main_production')
      await sink[3]!.setValue('gitlab')
      await sink[4]!.setValue('gitlab-clickhouse-gitlab')
      await sink[5]!.setValue('password')
    }

    /** The Siphon of that instance, carrying the status given, if any. */
    function siphonFixture(status?: Record<string, unknown>) {
      return {
        name: 'gitlab-siphon',
        namespace: 'gitlab-system',
        gitlabRef: 'gitlab',
        source: {
          host: 'gitlab-postgresql-rw',
          passwordSecretRef: { name: 'gitlab-siphon-postgresql', key: 'password' },
        },
        queue: { url: 'nats://nats:4222' },
        sink: {
          host: 'clickhouse',
          database: 'gitlab_clickhouse_main_production',
          username: 'gitlab',
          passwordSecretRef: { name: 'gitlab-clickhouse-gitlab', key: 'password' },
        },
        chart: { version: '1.21.0' },
        ...(status ? { status } : {}),
      }
    }

    /**
     * Answers the fetches of an instance being edited: the instance itself,
     * and the Siphon given. `null` answers the Siphon with the 404 an
     * instance that has none reads as.
     */
    function mockInstance(siphon: Record<string, unknown> | null) {
      const instance = {
        data: { name: 'gitlab', namespace: 'gitlab-system', chart: { version: '10.2.2' } },
        error: undefined,
      }
      const addon = siphon
        ? { data: siphon, error: undefined }
        : {
            data: undefined,
            error: { detail: 'Siphon resource not found' },
            response: { status: 404 },
          }

      mockApi.GET.mockImplementation((path: string) =>
        Promise.resolve(path.endsWith('/siphon') ? addon : instance),
      )
    }

    it('is configurable rather than a placeholder', async () => {
      const wrapper = await mountForm()

      expect(navButton(wrapper, 'Siphon').attributes('disabled')).toBeUndefined()

      await selectSection(wrapper, 'Siphon')
      expect(currentSection(wrapper)).toBe('Siphon')
    })

    it('unlocks its groups on enable', async () => {
      const wrapper = await mountForm()

      await selectSection(wrapper, 'Siphon')
      expect(group(wrapper, 'PostgreSQL source').attributes('disabled')).toBeDefined()

      await panel(wrapper).get('input[type="checkbox"]').setValue(true)
      expect(group(wrapper, 'PostgreSQL source').attributes('disabled')).toBeUndefined()
      expect(group(wrapper, 'ClickHouse sink').attributes('disabled')).toBeUndefined()
    })

    it('requires the three connections and a chart version once enabled', async () => {
      const wrapper = await mountForm()

      await fill(wrapper, 'Chart version', '10.2.2')
      await selectSection(wrapper, 'Siphon')
      await panel(wrapper).get('input[type="checkbox"]').setValue(true)
      await wrapper.get('form').trigger('submit')

      expect(currentSection(wrapper)).toBe('Siphon')
      expect(wrapper.text()).toContain('Siphon needs a chart version')
      expect(wrapper.text()).toContain('The PostgreSQL source needs a hostname')
      expect(wrapper.text()).toContain('Siphon needs the URL of a NATS server')
      expect(wrapper.text()).toContain('The ClickHouse sink needs a hostname')
      expect(mockApi.POST).not.toHaveBeenCalled()
    })

    // The CRD rejects the HTTP interface port, so the form does too.
    it('rejects the ClickHouse HTTP port', async () => {
      const wrapper = await mountForm()

      await fill(wrapper, 'Chart version', '10.2.2')
      await selectSection(wrapper, 'Siphon')
      await panel(wrapper).get('input[type="checkbox"]').setValue(true)
      await fillSiphon(wrapper)
      await group(wrapper, 'ClickHouse sink').findAll('input')[1]!.setValue('8123')

      await wrapper.get('form').trigger('submit')

      expect(currentSection(wrapper)).toBe('Siphon')
      expect(wrapper.text()).toContain('Port 8123 is the ClickHouse HTTP interface')
      expect(mockApi.POST).not.toHaveBeenCalled()
    })

    it('saves the Siphon under the instance once it is stored', async () => {
      mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })
      mockApi.PUT.mockResolvedValue({ data: undefined, error: undefined })

      const wrapper = await mountForm()

      await fill(wrapper, 'Chart version', '10.2.2')
      await selectSection(wrapper, 'Siphon')
      await panel(wrapper).get('input[type="checkbox"]').setValue(true)
      await fillSiphon(wrapper)

      await wrapper.get('form').trigger('submit')
      await flushPromises()

      expect(mockApi.POST).toHaveBeenCalled()
      expect(mockApi.PUT).toHaveBeenCalledWith(
        '/api/v1/namespaces/{namespace}/gitlabs/{name}/siphon',
        {
          params: { path: { namespace: 'gitlab-system', name: 'gitlab' } },
          body: {
            source: {
              host: 'gitlab-postgresql-rw.databases.svc.cluster.local',
              port: undefined,
              database: undefined,
              user: undefined,
              passwordSecretRef: { name: 'gitlab-siphon-postgresql', key: 'password' },
              sslMode: 'require',
              advisoryLockID: undefined,
            },
            queue: { url: 'nats://nats.nats.svc.cluster.local:4222', auth: undefined, tls: undefined },
            sink: {
              host: 'clickhouse.databases.svc.cluster.local',
              port: undefined,
              database: 'gitlab_clickhouse_main_production',
              username: 'gitlab',
              passwordSecretRef: { name: 'gitlab-clickhouse-gitlab', key: 'password' },
              ssl: false,
            },
            tables: { source: 'Auto', image: undefined, pullSecretRef: undefined },
            chart: { version: '1.21.0', values: {} },
          },
        },
      )
    })

    it('touches no Siphon endpoint while the add-on is off', async () => {
      mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })

      const wrapper = await mountForm()

      await fill(wrapper, 'Chart version', '10.2.2')
      await wrapper.get('form').trigger('submit')
      await flushPromises()

      expect(mockApi.PUT).not.toHaveBeenCalled()
      expect(mockApi.DELETE).not.toHaveBeenCalled()
    })

    it('loads the Siphon of an instance being edited', async () => {
      mockInstance(siphonFixture({ phase: 'Running', tableCount: 42, tablesSource: 'ConfigMap' }))

      const wrapper = await mountForm({ namespace: 'gitlab-system', name: 'gitlab' })

      expect(navButton(wrapper, 'Siphon').text()).toContain('On')

      await selectSection(wrapper, 'Siphon')
      expect((group(wrapper, 'Chart').get('input').element as HTMLInputElement).value).toBe('1.21.0')
      expect(panel(wrapper).text()).toContain('Running')
      expect(panel(wrapper).text()).toContain('42 (ConfigMap)')
    })

    // An instance without a Siphon is the normal case, not a failure.
    it('leaves the add-on off when the instance has none', async () => {
      mockInstance(null)

      const wrapper = await mountForm({ namespace: 'gitlab-system', name: 'gitlab' })

      expect(navButton(wrapper, 'Siphon').text()).not.toContain('On')
      expect(wrapper.text()).not.toContain('not found')
    })

    // A caller allowed on GitLab resources but not on Siphons can still edit
    // the instance; whether it has a Siphon is unknown, so the add-on is
    // locked rather than shown off and turned on over one that went unseen.
    it('locks the add-on when the Siphon cannot be read', async () => {
      mockInstance(null)
      const instance = mockApi.GET.getMockImplementation()!
      mockApi.GET.mockImplementation((path: string) =>
        path.endsWith('/siphon')
          ? Promise.resolve({
              data: undefined,
              error: { detail: 'siphons.apps.gitlab.com is forbidden' },
              response: { status: 403 },
            })
          : instance(path),
      )
      mockApi.PUT.mockResolvedValue({ data: undefined, error: undefined })

      const wrapper = await mountForm({ namespace: 'gitlab-system', name: 'gitlab' })

      expect(wrapper.find('[role="alert"]').exists()).toBe(false)
      expect(navButton(wrapper, 'Siphon').text()).toContain('Unknown')

      await selectSection(wrapper, 'Siphon')
      expect(panel(wrapper).get('input[type="checkbox"]').attributes('disabled')).toBeDefined()
      expect(panel(wrapper).text()).toContain('could not be read')

      await wrapper.get('form').trigger('submit')
      await flushPromises()

      // Only the instance is saved.
      expect(mockApi.PUT).toHaveBeenCalledTimes(1)
      expect(mockApi.PUT.mock.calls[0]![0]).not.toMatch(/\/siphon$/)
      expect(mockApi.DELETE).not.toHaveBeenCalled()
    })

    // Turning the add-on off deletes a resource, so it is confirmed first.
    it('deletes the Siphon when the add-on is turned off, once confirmed', async () => {
      mockInstance(siphonFixture())
      mockApi.PUT.mockResolvedValue({ data: undefined, error: undefined })
      mockApi.DELETE.mockResolvedValue({ data: undefined, error: undefined })
      vi.stubGlobal('confirm', vi.fn(() => true))

      const wrapper = await mountForm({ namespace: 'gitlab-system', name: 'gitlab' })

      await selectSection(wrapper, 'Siphon')
      await panel(wrapper).get('input[type="checkbox"]').setValue(false)
      await wrapper.get('form').trigger('submit')
      await flushPromises()

      expect(confirm).toHaveBeenCalled()
      expect(mockApi.DELETE).toHaveBeenCalledWith(
        '/api/v1/namespaces/{namespace}/gitlabs/{name}/siphon',
        { params: { path: { namespace: 'gitlab-system', name: 'gitlab' } } },
      )

      vi.unstubAllGlobals()
    })

    // The instance is stored before its Siphon, so a Siphon that fails leaves
    // it created. Submitting again has to update that instance rather than
    // create a second one under the same name, which the API server rejects.
    it('updates the instance it just created when the Siphon failed', async () => {
      mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })
      mockApi.PUT.mockResolvedValue({ data: undefined, error: { detail: 'no kind "Siphon" is registered' } })

      const wrapper = await mountForm()

      await fill(wrapper, 'Chart version', '10.2.2')
      await selectSection(wrapper, 'Siphon')
      await panel(wrapper).get('input[type="checkbox"]').setValue(true)
      await fillSiphon(wrapper)

      await wrapper.get('form').trigger('submit')
      await flushPromises()

      expect(mockApi.POST).toHaveBeenCalledTimes(1)
      expect(push).not.toHaveBeenCalled()
      expect(wrapper.text()).toContain('The instance was saved, but its Siphon was not.')
      // The form now edits what it created: the name is fixed, and the
      // button saves rather than creates.
      expect(wrapper.get('button[type="submit"]').text()).toBe('Save')
      expect(panel(wrapper).html()).not.toContain('Siphon needs a chart version')

      await wrapper.get('form').trigger('submit')
      await flushPromises()

      expect(mockApi.POST).toHaveBeenCalledTimes(1)
      expect(mockApi.PUT).toHaveBeenCalledWith('/api/v1/namespaces/{namespace}/gitlabs/{name}', {
        params: { path: { namespace: 'gitlab-system', name: 'gitlab' } },
        body: expect.objectContaining({ name: 'gitlab', namespace: 'gitlab-system' }),
      })
    })

    // Declining the confirmation is not a failure: the instance is saved and
    // the Siphon is left alone, which the form says rather than navigating
    // away from a checkbox that no longer matches the cluster.
    it('keeps the form open when the deletion is declined', async () => {
      mockInstance(siphonFixture())
      mockApi.PUT.mockResolvedValue({ data: undefined, error: undefined })
      vi.stubGlobal('confirm', vi.fn(() => false))

      const wrapper = await mountForm({ namespace: 'gitlab-system', name: 'gitlab' })

      await selectSection(wrapper, 'Siphon')
      await panel(wrapper).get('input[type="checkbox"]').setValue(false)
      await wrapper.get('form').trigger('submit')
      await flushPromises()

      expect(mockApi.DELETE).not.toHaveBeenCalled()
      expect(push).not.toHaveBeenCalled()
      expect(wrapper.text()).toContain('its Siphon kept')

      vi.unstubAllGlobals()
    })

    // The stores outlive the page, and a new instance fetches no Siphon of
    // its own: the one left by the instance edited before is not this one's.
    it('does not offer to delete the Siphon of the instance edited before', async () => {
      mockInstance(siphonFixture())
      await mountForm({ namespace: 'gitlab-system', name: 'gitlab' })

      mockApi.POST.mockResolvedValue({ data: undefined, error: undefined })
      vi.stubGlobal('confirm', vi.fn(() => true))

      const wrapper = await mountForm()

      await fill(wrapper, 'Chart version', '10.2.2')
      await wrapper.get('form').trigger('submit')
      await flushPromises()

      expect(confirm).not.toHaveBeenCalled()
      expect(mockApi.DELETE).not.toHaveBeenCalled()
      expect(push).toHaveBeenCalled()

      vi.unstubAllGlobals()
    })
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
