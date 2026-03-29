<script>
  import { onMount } from 'svelte'

  const apiBase = '/api'

  let patients = []
  let variants = []
  let jobs = []
  let similar = []
  let patientVariants = []
  let variantClinical = null
  let variantExternal = []
  let message = ''
  let token = ''
  let currentUser = null
  let authError = ''
  let authFieldErrors = {}
  let fieldErrors = {}
  let searchFieldErrors = {}

  let patientForm = {
    external_id: '',
    first_name: '',
    last_name: '',
    date_of_birth: '',
    sex: 'male',
    email: '',
  }

  let variantForm = {
    chromosome: '',
    position: '',
    reference_allele: '',
    alternate_allele: '',
    rs_id: '',
    genome_build: 'GRCh38',
    variant_type: 'SNV',
    patient_id: '',
  }

  let uploadPatientId = ''
  let uploadSampleId = ''
  let uploadGenomeBuild = 'GRCh38'
  let vcfFile
  let csvFile
  let fastqRead1
  let fastqRead2
  let fastqPatientId = ''
  let fastqSampleName = ''

  let similarBy = 'rsid'
  let similarQuery = {
    rs_id: '',
    gene_id: '',
    gene_symbol: '',
    chromosome: '',
    position: '',
    reference_allele: '',
    alternate_allele: '',
  }

  let selectedPatientId = ''
  let selectedVariantId = ''

  function authPageFromPath(pathname) {
    return pathname === '/register' ? 'register' : 'login'
  }

  function setAuthRoute(page, replace = false) {
    const path = page === 'register' ? '/register' : '/login'
    if (window.location.pathname !== path) {
      const fn = replace ? history.replaceState : history.pushState
      fn.call(history, {}, '', path)
    }
    authPage = page
  }

  function syncAuthFromLocation(replace = false) {
    const page = authPageFromPath(window.location.pathname)
    setAuthRoute(page, replace)
  }

  onMount(async () => {
    token = localStorage.getItem('dgv_token') || ''
    syncAuthFromLocation(true)
    window.addEventListener('popstate', () => syncAuthFromLocation(true))
    if (token) {
      await loadMe()
    }
    if (currentUser) {
      history.replaceState({}, '', '/')
      await refreshAll()
    }
  })

  async function refreshAll() {
    await Promise.all([loadPatients(), loadVariants(), loadJobs()])
  }

  async function api(path, options = {}) {
    const headers =
      options.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }
    if (token) headers.Authorization = `Bearer ${token}`
    const res = await fetch(`${apiBase}${path}`, {
      headers,
      ...options,
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      if (err.field_errors) {
        fieldErrors = err.field_errors
      }
      throw new Error(err.error || 'Request failed')
    }
    return res
  }

  async function loadMe() {
    try {
      const res = await api('/auth/me')
      currentUser = await res.json()
    } catch (err) {
      token = ''
      currentUser = null
      localStorage.removeItem('dgv_token')
    }
  }

  let authPage = 'login'
  let loginForm = { login: '', password: '' }
  let registerForm = { username: '', email: '', full_name: '', password: '' }

  function gotoAuth(page) {
    authError = ''
    authFieldErrors = {}
    setAuthRoute(page)
  }

  async function login() {
    authError = ''
    authFieldErrors = {}
    const fe = validateLogin(loginForm)
    if (Object.keys(fe).length) {
      authFieldErrors = fe
      return
    }
    const body = {
      password: loginForm.password,
    }
    if (loginForm.login.includes('@')) body.email = loginForm.login
    else body.username = loginForm.login
    try {
      const res = await fetch(`${apiBase}/auth/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        if (err.field_errors) authFieldErrors = err.field_errors
        throw new Error(err.error || 'Login failed')
      }
      const data = await res.json()
      token = data.token
      currentUser = data.user
      localStorage.setItem('dgv_token', token)
      history.replaceState({}, '', '/')
      await refreshAll()
    } catch (err) {
      authError = err.message
    }
  }

  async function register() {
    authError = ''
    authFieldErrors = {}
    const fe = validateRegister(registerForm)
    if (Object.keys(fe).length) {
      authFieldErrors = fe
      return
    }
    try {
      const res = await fetch(`${apiBase}/auth/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(registerForm),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }))
        if (err.field_errors) authFieldErrors = err.field_errors
        throw new Error(err.error || 'Register failed')
      }
      const data = await res.json()
      token = data.token
      currentUser = data.user
      localStorage.setItem('dgv_token', token)
      history.replaceState({}, '', '/')
      await refreshAll()
    } catch (err) {
      authError = err.message
    }
  }

  async function logout() {
    if (token) {
      await api('/auth/logout', { method: 'POST' }).catch(() => {})
    }
    token = ''
    currentUser = null
    localStorage.removeItem('dgv_token')
    setAuthRoute('login', true)
  }

  async function exportVariantsCsv() {
    try {
      const res = await api('/export/variants?format=csv')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'variants.csv'
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      message = 'Не удалось выгрузить CSV'
    }
  }

  async function loadPatients() {
    const res = await api('/patients')
    patients = await res.json()
  }

  async function loadVariants() {
    const res = await api('/variants')
    variants = await res.json()
  }

  async function loadJobs() {
    const res = await api('/jobs')
    jobs = await res.json()
  }

  async function loadPatientVariants() {
    if (!selectedPatientId) {
      patientVariants = []
      return
    }
    const res = await api(`/patient-variants?patient_id=${selectedPatientId}`)
    patientVariants = await res.json()
  }

  async function loadVariantDetail() {
    if (!selectedVariantId) {
      variantClinical = null
      variantExternal = []
      return
    }
    const [clinRes, extRes] = await Promise.all([
      api(`/variants/clinical?variant_id=${selectedVariantId}`),
      api(`/external/variants?variant_id=${selectedVariantId}`),
    ])
    variantClinical = await clinRes.json()
    variantExternal = await extRes.json()
  }

  async function createPatient() {
    message = ''
    fieldErrors = {}
    const fe = validatePatient(patientForm)
    if (Object.keys(fe).length) {
      fieldErrors = fe
      return
    }
    const body = { ...patientForm }
    if (body.email === '') delete body.email
    const res = await api('/patients', { method: 'POST', body: JSON.stringify(body) })
    await res.json()
    await loadPatients()
    message = 'Пациент добавлен'
  }

  async function createVariant() {
    message = ''
    fieldErrors = {}
    const fe = validateVariant(variantForm)
    if (Object.keys(fe).length) {
      fieldErrors = fe
      return
    }
    const body = { ...variantForm }
    body.position = Number(body.position)
    if (body.patient_id === '') delete body.patient_id
    if (body.rs_id === '') delete body.rs_id
    const res = await api('/variants', { method: 'POST', body: JSON.stringify(body) })
    await res.json()
    await loadVariants()
    message = 'Вариант добавлен'
  }

  async function uploadVCF() {
    message = ''
    fieldErrors = {}
    const fe = validateUploadIds()
    if (Object.keys(fe).length) {
      fieldErrors = fe
      return
    }
    if (!vcfFile) {
      fieldErrors = { vcf_file: 'Файл VCF обязателен' }
      return
    }
    if (!vcfFile) return
    const form = new FormData()
    form.append('file', vcfFile)
    const params = new URLSearchParams()
    if (uploadPatientId) params.set('patient_id', uploadPatientId)
    if (uploadSampleId) params.set('sample_id', uploadSampleId)
    if (uploadGenomeBuild) params.set('genome_build', uploadGenomeBuild)
    const res = await api(`/upload/vcf?${params}`, { method: 'POST', body: form })
    const data = await res.json()
    message = `VCF импортирован: ${data.variants_imported}`
    await loadVariants()
  }

  async function uploadCSV() {
    message = ''
    fieldErrors = {}
    const fe = validateUploadIds()
    if (Object.keys(fe).length) {
      fieldErrors = fe
      return
    }
    if (!csvFile) {
      fieldErrors = { csv_file: 'Файл CSV обязателен' }
      return
    }
    if (!csvFile) return
    const form = new FormData()
    form.append('file', csvFile)
    const params = new URLSearchParams()
    if (uploadPatientId) params.set('patient_id', uploadPatientId)
    if (uploadSampleId) params.set('sample_id', uploadSampleId)
    if (uploadGenomeBuild) params.set('genome_build', uploadGenomeBuild)
    const res = await api(`/upload/csv?${params}`, { method: 'POST', body: form })
    const data = await res.json()
    message = `CSV импортирован: ${data.variants_imported}`
    await loadVariants()
  }

  async function uploadFASTQ() {
    message = ''
    fieldErrors = {}
    const fe = {}
    if (!fastqPatientId) fe.fastq_patient_id = 'patient_id обязателен'
    else if (!isPositiveInt(fastqPatientId))
      fe.fastq_patient_id = 'patient_id должен быть положительным целым'
    if (!fastqRead1) fe.fastq_read1 = 'read1 обязателен'
    if (Object.keys(fe).length) {
      fieldErrors = fe
      return
    }
    const form = new FormData()
    form.append('read1', fastqRead1)
    if (fastqRead2) form.append('read2', fastqRead2)
    const params = new URLSearchParams()
    params.set('patient_id', fastqPatientId)
    if (fastqSampleName) params.set('sample_name', fastqSampleName)
    params.set('genome_build', uploadGenomeBuild)
    const res = await api(`/upload/fastq?${params}`, { method: 'POST', body: form })
    await res.json()
    message = 'FASTQ загружен, задача создана'
    await loadJobs()
  }

  async function searchSimilar() {
    message = ''
    searchFieldErrors = {}
    const fe = validateSimilar(similarBy, similarQuery)
    if (Object.keys(fe).length) {
      searchFieldErrors = fe
      return
    }
    const params = new URLSearchParams()
    params.set('by', similarBy)
    if (similarBy === 'rsid') params.set('rs_id', similarQuery.rs_id)
    if (similarBy === 'gene') {
      if (similarQuery.gene_id) params.set('gene_id', similarQuery.gene_id)
      if (similarQuery.gene_symbol) params.set('gene_symbol', similarQuery.gene_symbol)
    }
    if (similarBy === 'locus') {
      params.set('chromosome', similarQuery.chromosome)
      params.set('position', similarQuery.position)
      params.set('reference_allele', similarQuery.reference_allele)
      params.set('alternate_allele', similarQuery.alternate_allele)
    }
    const res = await api(`/variants/similar?${params}`)
    similar = await res.json()
  }

  function clinvarBadge(value) {
    if (!value) return 'neutral'
    const v = value.toLowerCase()
    if (v.includes('pathogenic')) return 'bad'
    if (v.includes('benign')) return 'good'
    return 'neutral'
  }

  async function enrichVariant() {
    if (!selectedVariantId) return
    message = ''
    await api('/variants/enrich', {
      method: 'POST',
      body: JSON.stringify({ variant_id: Number(selectedVariantId) }),
    })
    await loadVariantDetail()
    message = 'Внешние данные обновлены'
  }

  function validatePatient(p) {
    const fe = {}
    if (!p.external_id) fe.external_id = 'Обязательно'
    if (!p.first_name) fe.first_name = 'Обязательно'
    if (!p.last_name) fe.last_name = 'Обязательно'
    if (!p.date_of_birth) fe.date_of_birth = 'Обязательно'
    if (!p.sex) fe.sex = 'Обязательно'
    if (p.email && !p.email.includes('@')) fe.email = 'Некорректный email'
    return fe
  }

  function validateVariant(v) {
    const fe = {}
    if (v.patient_id && !isPositiveInt(v.patient_id)) {
      fe.patient_id = 'patient_id должен быть положительным целым'
    }
    if (!v.chromosome) fe.chromosome = 'Обязательно'
    if (!isPositiveInt(v.position)) fe.position = 'Должно быть положительным целым'
    if (!v.reference_allele) fe.reference_allele = 'Обязательно'
    if (!v.alternate_allele) fe.alternate_allele = 'Обязательно'
    if (!v.genome_build) fe.genome_build = 'Обязательно'
    if (!v.variant_type) fe.variant_type = 'Обязательно'
    return fe
  }

  function validateLogin(f) {
    const fe = {}
    if (!f.login) fe.login = 'Обязательно'
    if (!f.password) fe.password = 'Обязательно'
    return fe
  }

  function validateRegister(f) {
    const fe = {}
    if (!f.username) fe.username = 'Обязательно'
    if (!f.email) fe.email = 'Обязательно'
    else if (!f.email.includes('@')) fe.email = 'Некорректный email'
    if (!f.full_name) fe.full_name = 'Обязательно'
    if (!f.password) fe.password = 'Обязательно'
    return fe
  }

  function validateSimilar(mode, q) {
    const fe = {}
    if (mode === 'rsid') {
      if (!q.rs_id) fe.rs_id = 'rs_id обязателен'
    } else if (mode === 'gene') {
      if (!q.gene_id && !q.gene_symbol) {
        fe.gene_id = 'Укажите gene_id или gene_symbol'
        fe.gene_symbol = 'Укажите gene_id или gene_symbol'
      }
    } else if (mode === 'locus') {
      if (!q.chromosome) fe.chromosome = 'Обязательно'
      if (!isPositiveInt(q.position)) fe.position = 'Должно быть положительным целым'
      if (!q.reference_allele) fe.reference_allele = 'Обязательно'
      if (!q.alternate_allele) fe.alternate_allele = 'Обязательно'
    }
    return fe
  }

  function validateUploadIds() {
    const fe = {}
    if (uploadPatientId && !isPositiveInt(uploadPatientId)) {
      fe.upload_patient_id = 'patient_id должен быть положительным целым'
    }
    if (uploadSampleId && !isPositiveInt(uploadSampleId)) {
      fe.upload_sample_id = 'sample_id должен быть положительным целым'
    }
    return fe
  }

  function isPositiveInt(value) {
    const num = Number(value)
    return Number.isInteger(num) && num > 0
  }

  function clearError(field) {
    if (fieldErrors[field]) {
      const next = { ...fieldErrors }
      delete next[field]
      fieldErrors = next
    }
  }

  function clearSearchError(field) {
    if (searchFieldErrors[field]) {
      const next = { ...searchFieldErrors }
      delete next[field]
      searchFieldErrors = next
    }
  }
</script>

<div class="page">
  <header class="topbar">
    <div class="brand">
      <div class="badge">DGV</div>
      <div>
        <h1>Кабинет врача‑генетика</h1>
        <p>Управление пациентами, вариантами, импорт и анализ.</p>
      </div>
    </div>
    <div class="actions">
      {#if currentUser}
        <span class="user">{currentUser.full_name} ({currentUser.role})</span>
        <button class="ghost" on:click={refreshAll}>Обновить</button>
        <button class="ghost" on:click={exportVariantsCsv}>Экспорт вариантов CSV</button>
        <button class="ghost" on:click={logout}>Выход</button>
      {/if}
    </div>
  </header>

  {#if !currentUser}
    <section class="card auth">
      <div class="auth-header">
        <h2>{authPage === 'login' ? 'Вход' : 'Регистрация'}</h2>
        <div class="auth-switch">
          {#if authPage === 'login'}
            <button class="link" on:click={() => gotoAuth('register')}>
              Создать аккаунт
            </button>
          {:else}
            <button class="link" on:click={() => gotoAuth('login')}>
              Уже есть аккаунт
            </button>
          {/if}
        </div>
      </div>
      {#if authError}
        <div class="notice">{authError}</div>
      {/if}
      {#if authPage === 'login'}
        <div class="form">
          <input
            class:error-field={authFieldErrors.login || authFieldErrors.email || authFieldErrors.username}
            placeholder="Email или username"
            bind:value={loginForm.login}
            on:input={() => (authFieldErrors = { ...authFieldErrors, login: undefined, email: undefined, username: undefined })}
          />
          {#if authFieldErrors.login}<div class="error">{authFieldErrors.login}</div>{/if}
          {#if authFieldErrors.email}<div class="error">{authFieldErrors.email}</div>{/if}
          <input
            class:error-field={authFieldErrors.password}
            type="password"
            placeholder="Пароль"
            bind:value={loginForm.password}
            on:input={() => (authFieldErrors = { ...authFieldErrors, password: undefined })}
          />
          {#if authFieldErrors.password}<div class="error">{authFieldErrors.password}</div>{/if}
          <button on:click={login}>Войти</button>
        </div>
      {:else}
        <div class="form">
          <input
            class:error-field={authFieldErrors.username}
            placeholder="Username"
            bind:value={registerForm.username}
            on:input={() => (authFieldErrors = { ...authFieldErrors, username: undefined })}
          />
          {#if authFieldErrors.username}<div class="error">{authFieldErrors.username}</div>{/if}
          <input
            class:error-field={authFieldErrors.email}
            placeholder="Email"
            bind:value={registerForm.email}
            on:input={() => (authFieldErrors = { ...authFieldErrors, email: undefined })}
          />
          {#if authFieldErrors.email}<div class="error">{authFieldErrors.email}</div>{/if}
          <input
            class:error-field={authFieldErrors.full_name}
            placeholder="ФИО"
            bind:value={registerForm.full_name}
            on:input={() => (authFieldErrors = { ...authFieldErrors, full_name: undefined })}
          />
          {#if authFieldErrors.full_name}<div class="error">{authFieldErrors.full_name}</div>{/if}
          <input
            class:error-field={authFieldErrors.password}
            type="password"
            placeholder="Пароль"
            bind:value={registerForm.password}
            on:input={() => (authFieldErrors = { ...authFieldErrors, password: undefined })}
          />
          {#if authFieldErrors.password}<div class="error">{authFieldErrors.password}</div>{/if}
          <button on:click={register}>Зарегистрироваться</button>
        </div>
      {/if}
    </section>
  {:else}

  {#if message}
    <div class="notice">{message}</div>
  {/if}

  <div class="grid">
    <section class="card">
      <h2>Пациенты</h2>
      <div class="form">
        <input
          class:error-field={fieldErrors.external_id}
          placeholder="External ID"
          bind:value={patientForm.external_id}
          on:input={() => clearError('external_id')}
        />
        {#if fieldErrors.external_id}<div class="error">{fieldErrors.external_id}</div>{/if}
        <input
          class:error-field={fieldErrors.first_name}
          placeholder="Имя"
          bind:value={patientForm.first_name}
          on:input={() => clearError('first_name')}
        />
        {#if fieldErrors.first_name}<div class="error">{fieldErrors.first_name}</div>{/if}
        <input
          class:error-field={fieldErrors.last_name}
          placeholder="Фамилия"
          bind:value={patientForm.last_name}
          on:input={() => clearError('last_name')}
        />
        {#if fieldErrors.last_name}<div class="error">{fieldErrors.last_name}</div>{/if}
        <input
          class:error-field={fieldErrors.date_of_birth}
          type="date"
          bind:value={patientForm.date_of_birth}
          on:input={() => clearError('date_of_birth')}
        />
        {#if fieldErrors.date_of_birth}<div class="error">{fieldErrors.date_of_birth}</div>{/if}
        <select
          class:error-field={fieldErrors.sex}
          bind:value={patientForm.sex}
          on:change={() => clearError('sex')}
        >
          <option value="male">male</option>
          <option value="female">female</option>
          <option value="other">other</option>
        </select>
        {#if fieldErrors.sex}<div class="error">{fieldErrors.sex}</div>{/if}
        <input
          class:error-field={fieldErrors.email}
          placeholder="Email"
          bind:value={patientForm.email}
          on:input={() => clearError('email')}
        />
        {#if fieldErrors.email}<div class="error">{fieldErrors.email}</div>{/if}
        <button on:click={createPatient}>Добавить пациента</button>
        <p class="hint">Обязательные: external_id, имя, фамилия, дата рождения, пол.</p>
      </div>
      <div class="form">
        <select bind:value={selectedPatientId} on:change={loadPatientVariants}>
          <option value="">Выбрать пациента для карточки</option>
          {#each patients as p}
            <option value={p.patient_id}>{p.external_id} — {p.last_name} {p.first_name}</option>
          {/each}
        </select>
      </div>
      <div class="table">
        <div class="row head">
          <div>ID</div>
          <div>External</div>
          <div>ФИО</div>
        </div>
        {#each patients as p}
          <div class="row">
            <div>{p.patient_id}</div>
            <div>{p.external_id}</div>
            <div>{p.last_name} {p.first_name}</div>
          </div>
        {/each}
      </div>
    </section>

    <section class="card">
      <h2>Карточка варианта</h2>
      <div class="form">
        <select bind:value={selectedVariantId} on:change={loadVariantDetail}>
          <option value="">Выбрать вариант</option>
          {#each variants as v}
            <option value={v.variant_id}>
              {v.variant_id} — {v.chromosome}:{v.position} {v.reference_allele}&gt;{v.alternate_allele}
            </option>
          {/each}
        </select>
        <button class="ghost" on:click={enrichVariant} disabled={!selectedVariantId}>
          Обогатить из публичных источников
        </button>
      </div>
      {#if !selectedVariantId}
        <p class="muted">Выберите вариант, чтобы увидеть клиническую сводку.</p>
      {:else}
        <div class="table">
          <div class="row head">
            <div>ClinVar</div>
            <div>Заболевания</div>
            <div>ID</div>
          </div>
          <div class="row">
            <div>
              <span class={`tag ${clinvarBadge(variantClinical?.clinvar_significance)}`}>
                {variantClinical?.clinvar_significance || 'нет'}
              </span>
            </div>
            <div>{(variantClinical?.diseases || []).join(', ') || '-'}</div>
            <div>{variantClinical?.clinvar_id || '-'}</div>
          </div>
        </div>
        <div class="panel">
          <h3>Внешние данные (MyVariant)</h3>
          {#if variantExternal.length === 0}
            <p class="muted">Нет сохранённых данных.</p>
          {:else}
            <pre>{JSON.stringify(variantExternal[0].payload, null, 2)}</pre>
          {/if}
        </div>
      {/if}
    </section>

    <section class="card">
      <h2>Карточка пациента</h2>
      {#if selectedPatientId === ''}
        <p class="muted">Выберите пациента, чтобы увидеть его варианты и клиническую значимость.</p>
      {:else}
        <div class="table">
          <div class="row head">
            <div>Вариант</div>
            <div>Ген</div>
            <div>ClinVar</div>
          </div>
          {#each patientVariants as pv}
            <div class="row">
              <div>{pv.chromosome}:{pv.position} {pv.reference_allele}&gt;{pv.alternate_allele}</div>
              <div>{pv.gene_symbol || '-'}</div>
              <div>
                <span class={`tag ${clinvarBadge(pv.clinvar_significance)}`}>
                  {pv.clinvar_significance || 'нет'}
                </span>
                {#if pv.diseases && pv.diseases.length}
                  <div class="diseases">{pv.diseases.join(', ')}</div>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </section>

    <section class="card">
      <h2>Варианты</h2>
      <div class="form">
        <input
          class:error-field={fieldErrors.patient_id}
          placeholder="patient_id (опц.)"
          bind:value={variantForm.patient_id}
          on:input={() => clearError('patient_id')}
        />
        {#if fieldErrors.patient_id}<div class="error">{fieldErrors.patient_id}</div>{/if}
        <input
          class:error-field={fieldErrors.chromosome}
          placeholder="chr"
          bind:value={variantForm.chromosome}
          on:input={() => clearError('chromosome')}
        />
        {#if fieldErrors.chromosome}<div class="error">{fieldErrors.chromosome}</div>{/if}
        <input
          class:error-field={fieldErrors.position}
          placeholder="position"
          bind:value={variantForm.position}
          on:input={() => clearError('position')}
        />
        {#if fieldErrors.position}<div class="error">{fieldErrors.position}</div>{/if}
        <input
          class:error-field={fieldErrors.reference_allele}
          placeholder="ref"
          bind:value={variantForm.reference_allele}
          on:input={() => clearError('reference_allele')}
        />
        {#if fieldErrors.reference_allele}<div class="error">{fieldErrors.reference_allele}</div>{/if}
        <input
          class:error-field={fieldErrors.alternate_allele}
          placeholder="alt"
          bind:value={variantForm.alternate_allele}
          on:input={() => clearError('alternate_allele')}
        />
        {#if fieldErrors.alternate_allele}<div class="error">{fieldErrors.alternate_allele}</div>{/if}
        <input placeholder="rs_id" bind:value={variantForm.rs_id} />
        <select
          class:error-field={fieldErrors.genome_build}
          bind:value={variantForm.genome_build}
          on:change={() => clearError('genome_build')}
        >
          <option>GRCh38</option>
          <option>GRCh37</option>
        </select>
        {#if fieldErrors.genome_build}<div class="error">{fieldErrors.genome_build}</div>{/if}
        <select
          class:error-field={fieldErrors.variant_type}
          bind:value={variantForm.variant_type}
          on:change={() => clearError('variant_type')}
        >
          <option>SNV</option>
          <option>INS</option>
          <option>DEL</option>
          <option>INDEL</option>
          <option>CNV</option>
          <option>SV</option>
        </select>
        {#if fieldErrors.variant_type}<div class="error">{fieldErrors.variant_type}</div>{/if}
        <button on:click={createVariant}>Добавить вариант</button>
        <p class="hint">Обязательные: chr, position, ref, alt, genome_build, variant_type.</p>
      </div>
      <div class="table">
        <div class="row head">
          <div>ID</div>
          <div>Локус</div>
          <div>RS</div>
        </div>
        {#each variants as v}
          <div class="row">
            <div>{v.variant_id}</div>
            <div>{v.chromosome}:{v.position} {v.reference_allele}&gt;{v.alternate_allele}</div>
            <div>{v.rs_id || '-'}</div>
          </div>
        {/each}
      </div>
    </section>

    <section class="card">
      <h2>Импорт VCF/CSV</h2>
      <div class="form">
        <input
          class:error-field={fieldErrors.upload_patient_id}
          placeholder="patient_id (опционально)"
          bind:value={uploadPatientId}
          on:input={() => clearError('upload_patient_id')}
        />
        {#if fieldErrors.upload_patient_id}<div class="error">{fieldErrors.upload_patient_id}</div>{/if}
        <input
          class:error-field={fieldErrors.upload_sample_id}
          placeholder="sample_id (опционально)"
          bind:value={uploadSampleId}
          on:input={() => clearError('upload_sample_id')}
        />
        {#if fieldErrors.upload_sample_id}<div class="error">{fieldErrors.upload_sample_id}</div>{/if}
        <select bind:value={uploadGenomeBuild}>
          <option>GRCh38</option>
          <option>GRCh37</option>
        </select>
        <div class="file">
          <label>VCF</label>
          <input
            class:error-field={fieldErrors.vcf_file}
            type="file"
            on:change={(e) => {
              vcfFile = e.target.files[0]
              clearError('vcf_file')
            }}
          />
          <button on:click={uploadVCF}>Загрузить VCF</button>
          {#if fieldErrors.vcf_file}<div class="error">{fieldErrors.vcf_file}</div>{/if}
        </div>
        <div class="file">
          <label>CSV</label>
          <input
            class:error-field={fieldErrors.csv_file}
            type="file"
            on:change={(e) => {
              csvFile = e.target.files[0]
              clearError('csv_file')
            }}
          />
          <button on:click={uploadCSV}>Загрузить CSV</button>
          {#if fieldErrors.csv_file}<div class="error">{fieldErrors.csv_file}</div>{/if}
        </div>
      </div>
    </section>

    <section class="card">
      <h2>Импорт FASTQ и SNP</h2>
      <div class="form">
        <input
          class:error-field={fieldErrors.fastq_patient_id}
          placeholder="patient_id"
          bind:value={fastqPatientId}
          on:input={() => clearError('fastq_patient_id')}
        />
        {#if fieldErrors.fastq_patient_id}<div class="error">{fieldErrors.fastq_patient_id}</div>{/if}
        <input placeholder="sample_name (опционально)" bind:value={fastqSampleName} />
        <select bind:value={uploadGenomeBuild}>
          <option>GRCh38</option>
          <option>GRCh37</option>
        </select>
        <div class="file">
          <label>read1.fastq</label>
          <input
            class:error-field={fieldErrors.fastq_read1}
            type="file"
            on:change={(e) => {
              fastqRead1 = e.target.files[0]
              clearError('fastq_read1')
            }}
          />
          {#if fieldErrors.fastq_read1}<div class="error">{fieldErrors.fastq_read1}</div>{/if}
        </div>
        <div class="file">
          <label>read2.fastq (опционально)</label>
          <input type="file" on:change={(e) => (fastqRead2 = e.target.files[0])} />
        </div>
        <button on:click={uploadFASTQ}>Создать задачу</button>
        <p class="hint">Обязательные: patient_id и read1 FASTQ.</p>
      </div>
    </section>

    <section class="card">
      <h2>Похожие мутации</h2>
      <div class="form">
        {#if searchFieldErrors && Object.keys(searchFieldErrors).length}
          <div class="error">Исправьте ошибки в фильтрах перед поиском.</div>
        {/if}
        <select bind:value={similarBy} on:change={() => (searchFieldErrors = {})}>
          <option value="rsid">По rs_id</option>
          <option value="gene">По гену</option>
          <option value="locus">По локусу</option>
        </select>
        {#if similarBy === 'rsid'}
          <input
            class:error-field={searchFieldErrors.rs_id}
            placeholder="rs_id"
            bind:value={similarQuery.rs_id}
            on:input={() => clearSearchError('rs_id')}
          />
          {#if searchFieldErrors.rs_id}<div class="error">{searchFieldErrors.rs_id}</div>{/if}
        {:else if similarBy === 'gene'}
          <input
            class:error-field={searchFieldErrors.gene_id}
            placeholder="gene_id"
            bind:value={similarQuery.gene_id}
            on:input={() => clearSearchError('gene_id')}
          />
          {#if searchFieldErrors.gene_id}<div class="error">{searchFieldErrors.gene_id}</div>{/if}
          <input
            class:error-field={searchFieldErrors.gene_symbol}
            placeholder="gene_symbol"
            bind:value={similarQuery.gene_symbol}
            on:input={() => clearSearchError('gene_symbol')}
          />
          {#if searchFieldErrors.gene_symbol}<div class="error">{searchFieldErrors.gene_symbol}</div>{/if}
        {:else}
          <input
            class:error-field={searchFieldErrors.chromosome}
            placeholder="chromosome"
            bind:value={similarQuery.chromosome}
            on:input={() => clearSearchError('chromosome')}
          />
          {#if searchFieldErrors.chromosome}<div class="error">{searchFieldErrors.chromosome}</div>{/if}
          <input
            class:error-field={searchFieldErrors.position}
            placeholder="position"
            bind:value={similarQuery.position}
            on:input={() => clearSearchError('position')}
          />
          {#if searchFieldErrors.position}<div class="error">{searchFieldErrors.position}</div>{/if}
          <input
            class:error-field={searchFieldErrors.reference_allele}
            placeholder="reference_allele"
            bind:value={similarQuery.reference_allele}
            on:input={() => clearSearchError('reference_allele')}
          />
          {#if searchFieldErrors.reference_allele}
            <div class="error">{searchFieldErrors.reference_allele}</div>
          {/if}
          <input
            class:error-field={searchFieldErrors.alternate_allele}
            placeholder="alternate_allele"
            bind:value={similarQuery.alternate_allele}
            on:input={() => clearSearchError('alternate_allele')}
          />
          {#if searchFieldErrors.alternate_allele}
            <div class="error">{searchFieldErrors.alternate_allele}</div>
          {/if}
        {/if}
        <button on:click={searchSimilar}>Найти</button>
      </div>
      <div class="table">
        <div class="row head">
          <div>Вариант</div>
          <div>Ген</div>
          <div>ClinVar</div>
        </div>
        {#each similar as s}
          <div class="row">
            <div>
              {s.chromosome}:{s.position} {s.reference_allele}&gt;{s.alternate_allele}
              {#if s.rs_id} · {s.rs_id}{/if}
            </div>
            <div>{s.gene_symbol || '-'}</div>
            <div>{s.clinvar_significance || '-'}</div>
          </div>
        {/each}
      </div>
    </section>

    <section class="card">
      <h2>Очередь FASTQ</h2>
      <div class="table">
        <div class="row head">
          <div>ID</div>
          <div>Статус</div>
          <div>VCF</div>
        </div>
        {#each jobs as j}
          <div class="row">
            <div>{j.job_id}</div>
            <div>{j.status}</div>
            <div>{j.vcf_path || '-'}</div>
          </div>
        {/each}
      </div>
    </section>
  </div>
  {/if}
</div>
