<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ClipboardCheck, GitCompareArrows, Plus, Play, LockKeyhole, ListChecks, TriangleAlert, CircleCheck, CircleX, RotateCcw, X } from 'lucide-vue-next'
import PageHeader from '@/components/common/PageHeader.vue'
import ReviewDialog from '@/components/common/ReviewDialog.vue'
import { useCaseStore } from '@/stores/cases'
import { useRouteStore } from '@/stores/routes'
import { useTraceStore } from '@/stores/traces'
import { useAuth } from '@/hooks/useAuth'
import axios from 'axios'
import { CASE_STATUSES, caseStatusLabel, type BatchCaseConflict, type BatchCaseResult, type LocalizationCase } from '@/types/case'

const cases = useCaseStore(); const routes = useRouteStore(); const traces = useTraceStore(); const auth = useAuth()
const status = ref(''); const createOpen = ref(false); const reviewOpen = ref(false); const current = ref<LocalizationCase | null>(null); const busy = ref(false)
const tableRef = ref()
const selected = ref<LocalizationCase[]>([])
const batchForm = reactive({ distance_tolerance_m: 25, loss_increase_db: 0.5 })
const batchFilter = ref<'' | 'succeeded' | 'failed'>('')
const form = reactive({ route_id: 0, baseline_trace_id: 0, current_trace_id: 0, distance_tolerance_m: 25, loss_increase_db: 0.5 })
const routeTraces = computed(() => traces.items.filter((item) => item.route_id === form.route_id))
const selectedDrafts = computed(() => selected.value.filter((item) => item.case_status === 'draft'))
const filteredBatchResults = computed<BatchCaseResult[]>(() => {
  const results = cases.batchResult?.results ?? []
  return batchFilter.value ? results.filter((item) => item.outcome === batchFilter.value) : results
})
function routeCode(routeID: number) { return routes.items.find((item) => item.id === routeID)?.route_code ?? `#${routeID}` }
async function search() { selected.value = []; await cases.fetch({ status: status.value || undefined, page_size: 100 }) }
function onSelectionChange(rows: LocalizationCase[]) { selected.value = rows }
function chooseRoute() { const route = routes.items.find((item) => item.id === form.route_id); form.baseline_trace_id = route?.baseline_trace_id ?? 0; form.current_trace_id = routeTraces.value.find((trace) => trace.id !== form.baseline_trace_id)?.id ?? 0 }
async function create() { busy.value = true; try { await cases.create(form); createOpen.value = false; ElMessage.success('定位案例已建立') } finally { busy.value = false } }
async function analyze(item: LocalizationCase) { busy.value = true; try { await cases.analyze(item.id, {}); ElMessage.success('基线差异分析已完成') } finally { busy.value = false } }
function confirm(item: LocalizationCase) { current.value = item; reviewOpen.value = true }
async function submitReview(value: Record<string, unknown>) { if (!current.value) return; busy.value = true; try { await cases.confirm(current.value.id, value); reviewOpen.value = false; ElMessage.success('人工结论已确认') } finally { busy.value = false } }
async function close(item: LocalizationCase) { await ElMessageBox.confirm('关闭后案例不可再修改，确认继续？', '关闭案例', { type: 'warning', confirmButtonText: '确认关闭', cancelButtonText: '取消' }); await cases.close(item.id, item.version); ElMessage.success('案例已关闭') }
function conflictText(conflict: BatchCaseConflict) {
  const label = conflict.status ? caseStatusLabel[conflict.status as LocalizationCase['case_status']] ?? conflict.status : ''
  if (conflict.reason === 'NOT_DRAFT') return `<li><strong>#${conflict.case_id}</strong>：当前状态为「${label}」，只有草稿可以批量重算，已跳过</li>`
  if (conflict.reason === 'NOT_FOUND') return `<li><strong>#${conflict.case_id}</strong>：案例不存在或已被删除，已跳过</li>`
  return `<li><strong>#${conflict.case_id}</strong>：在勾选列表中重复出现，已自动去重</li>`
}
async function runBatch(retryIDs?: number[]) {
  const ids = retryIDs ?? selectedDrafts.value.map((item) => item.id)
  if (ids.length === 0) { ElMessage.warning('请先勾选草稿状态的案例'); return }
  try {
    await cases.runBatch({ case_ids: ids, distance_tolerance_m: batchForm.distance_tolerance_m, loss_increase_db: batchForm.loss_increase_db })
    tableRef.value?.clearSelection()
    batchFilter.value = ''
    await search()
    const result = cases.batchResult
    if (result) { result.failed > 0 ? ElMessage.warning(`本批 ${result.total} 条：${result.succeeded} 成功，${result.failed} 失败`) : ElMessage.success(`本批 ${result.total} 条全部重算成功`) }
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.status === 409) {
      const conflicts = (error.response.data.error.details?.conflicts ?? []) as BatchCaseConflict[]
      await ElMessageBox.alert(`<ul style="margin:8px 0 0;padding-left:18px;line-height:1.9">${conflicts.map(conflictText).join('')}</ul><p style="margin:10px 0 0;color:var(--text-muted)">本批尚未执行，任何案例都没有改动。请处理上述冲突后重新提交。</p>`, '批量提交被拦截：存在冲突项', { dangerouslyUseHTMLString: true, confirmButtonText: '知道了', type: 'warning' })
    } else if (axios.isAxiosError(error)) {
      const info = error.response?.data?.error
      ElMessage.error(`${info?.message ?? '批量重算未完成'} [${info?.code ?? 'NETWORK_ERROR'}]`)
    }
  }
}
watch(status, () => { tableRef.value?.clearSelection() })
onMounted(async () => { await Promise.all([routes.fetch({ page_size: 100 }), traces.fetch({ page_size: 100 }), search()]); if (routes.items[0]) { form.route_id = routes.items[0].id; chooseRoute() } })
</script>

<template>
  <PageHeader title="定位案例" eyebrow="LOCALIZATION CASES" description="基线差异仅作分析参考，由复核人员形成最终结论。"><el-button v-if="auth.canAnalyze()" type="primary" @click="createOpen = true"><Plus :size="16" />新建案例</el-button></PageHeader>
  <section class="content-band">
    <div class="case-toolbar"><div class="state-track"><span v-for="(value,index) in CASE_STATUSES" :key="value"><i>{{ index+1 }}</i>{{ caseStatusLabel[value] }}</span></div><el-select v-model="status" clearable placeholder="全部状态" style="width:150px" @change="search"><el-option v-for="value in CASE_STATUSES" :key="value" :label="caseStatusLabel[value]" :value="value" /></el-select></div>
    <div v-if="auth.canAnalyze()" class="batch-bench">
      <div class="bench-head"><ListChecks :size="16" /><strong>批量重算工作台</strong><span class="bench-count">已勾选草稿 <b>{{ selectedDrafts.length }}</b> 条<span v-if="selected.length !== selectedDrafts.length" class="bench-note">（另有 {{ selected.length - selectedDrafts.length }} 条非草稿不可选）</span></span></div>
      <div class="bench-controls"><el-form-item label="距离容差 m" class="bench-field"><el-input-number v-model="batchForm.distance_tolerance_m" :min="0.1" :max="1000" :precision="2" /></el-form-item><el-form-item label="损耗增量阈值 dB" class="bench-field"><el-input-number v-model="batchForm.loss_increase_db" :min="0.1" :max="20" :step="0.1" :precision="2" /></el-form-item><el-button type="primary" :loading="cases.batchRunning" :disabled="selectedDrafts.length === 0 || batchForm.distance_tolerance_m == null || batchForm.loss_increase_db == null" @click="runBatch()"><Play :size="15" />统一提交重算（{{ selectedDrafts.length }}）</el-button><span class="bench-hint">仅草稿案例可提交；已确认、已关闭等状态会被拦截，冲突项先提示后执行。</span></div>
    </div>
    <div class="data-surface">
      <el-table ref="tableRef" v-loading="cases.loading" :data="cases.items" row-key="id" @selection-change="onSelectionChange">
        <el-table-column v-if="auth.canAnalyze()" type="selection" width="42" reserve-selection :selectable="(row: LocalizationCase) => row.case_status === 'draft'" />
        <el-table-column prop="id" label="案例" width="80"><template #default="scope"><strong>#{{ scope.row.id }}</strong></template></el-table-column>
        <el-table-column label="线路" width="125"><template #default="scope">{{ routeCode(scope.row.route_id) }}</template></el-table-column>
        <el-table-column label="对比轨迹" min-width="150"><template #default="scope"><span class="trace-pair">#{{ scope.row.baseline_trace_id }} <GitCompareArrows :size="14" /> #{{ scope.row.current_trace_id }}</span></template></el-table-column>
        <el-table-column label="状态" width="115"><template #default="scope"><span class="status-pill" :class="scope.row.case_status">{{ caseStatusLabel[scope.row.case_status as LocalizationCase['case_status']] }}</span></template></el-table-column>
        <el-table-column label="估算位置" width="150"><template #default="scope"><template v-if="scope.row.estimated_distance_m != null"><strong>{{ scope.row.estimated_distance_m.toFixed(2) }} m</strong><small class="uncertainty">± {{ scope.row.uncertainty_m?.toFixed(2) }} m</small></template><span v-else class="muted">待分析</span></template></el-table-column>
        <el-table-column prop="conclusion" label="复核结论" min-width="230"><template #default="scope"><span v-if="scope.row.conclusion" class="conclusion">{{ scope.row.conclusion }}</span><span v-else class="muted">尚未确认</span></template></el-table-column>
        <el-table-column label="操作" width="190" fixed="right"><template #default="scope"><el-button v-if="scope.row.case_status==='draft' && auth.canAnalyze()" link type="primary" :loading="busy" @click="analyze(scope.row)"><Play :size="14" />分析</el-button><el-button v-if="scope.row.case_status==='pending_review' && auth.canReview()" link type="primary" @click="confirm(scope.row)"><ClipboardCheck :size="14" />确认</el-button><el-button v-if="scope.row.case_status==='confirmed' && auth.canReview()" link type="danger" @click="close(scope.row)"><LockKeyhole :size="14" />关闭</el-button><span v-if="scope.row.case_status==='closed'" class="muted">已归档</span></template></el-table-column>
        <template #empty><div class="empty-state"><div><ClipboardCheck :size="34" /><strong>尚无定位案例</strong><span>选择同线路基线与当前轨迹建立对比。</span></div></div></template>
      </el-table>
    </div>
    <div v-if="cases.batchResult" class="batch-results">
      <div class="results-head">
        <div class="results-summary"><ListChecks :size="16" /><strong>本批重算结果</strong><span class="batch-id">批次 {{ cases.batchResult.batch_id }}</span><el-tag type="success" effect="plain" size="small"><CircleCheck :size="13" /> 成功 {{ cases.batchResult.succeeded }}</el-tag><el-tag v-if="cases.batchResult.failed > 0" type="danger" effect="plain" size="small"><CircleX :size="13" /> 失败 {{ cases.batchResult.failed }}</el-tag><el-tag v-else type="info" effect="plain" size="small">共 {{ cases.batchResult.total }} 条</el-tag></div>
        <div class="results-actions"><el-radio-group v-model="batchFilter" size="small"><el-radio-button label="">全部</el-radio-button><el-radio-button label="succeeded">仅成功</el-radio-button><el-radio-button label="failed">仅失败</el-radio-button></el-radio-group><el-button v-if="cases.batchResult.failed > 0" size="small" :loading="cases.batchRunning" @click="runBatch(cases.batchResult!.results.filter((item) => item.outcome === 'failed').map((item) => item.case_id))"><RotateCcw :size="14" />退回失败项重算</el-button><el-button size="small" text @click="cases.clearBatch()"><X :size="14" />清除结果</el-button></div>
      </div>
      <el-table :data="filteredBatchResults" size="small" row-key="case_id" class="results-table">
        <el-table-column prop="case_id" label="案例" width="80"><template #default="scope"><strong>#{{ scope.row.case_id }}</strong></template></el-table-column>
        <el-table-column label="线路" width="120"><template #default="scope">{{ routeCode(scope.row.route_id) }}</template></el-table-column>
        <el-table-column label="结果" width="100"><template #default="scope"><span v-if="scope.row.outcome === 'succeeded'" class="result-ok"><CircleCheck :size="14" />成功</span><span v-else class="result-fail"><CircleX :size="14" />失败</span></template></el-table-column>
        <el-table-column label="当前状态" width="105"><template #default="scope"><span class="status-pill" :class="scope.row.case_status">{{ caseStatusLabel[scope.row.case_status as LocalizationCase['case_status']] }}</span></template></el-table-column>
        <el-table-column label="估算位置" width="170"><template #default="scope"><template v-if="scope.row.estimated_distance_m != null"><strong>{{ scope.row.estimated_distance_m.toFixed(2) }} m</strong><small class="uncertainty">± {{ scope.row.uncertainty_m?.toFixed(2) }} m · {{ scope.row.difference_count }} 项差异</small></template><span class="muted">无主要差异位置</span></template></el-table-column>
        <el-table-column label="失败原因" min-width="240"><template #default="scope"><template v-if="scope.row.outcome === 'failed'"><span class="fail-reason"><TriangleAlert :size="14" />{{ scope.row.error_message }}<el-tag size="small" type="danger" effect="plain" class="error-code">{{ scope.row.error_code }}</el-tag></span></template><span v-else class="muted">已进入待复核</span></template></el-table-column>
        <template #empty><div class="muted results-empty">本批结果中没有匹配当前筛选的条目。</div></template>
      </el-table>
    </div>
  </section>
  <el-dialog v-model="createOpen" title="建立基线对比案例" width="min(620px, calc(100vw - 28px))"><el-form label-position="top"><el-form-item label="线路"><el-select v-model="form.route_id" style="width:100%" @change="chooseRoute"><el-option v-for="route in routes.items" :key="route.id" :label="`${route.route_code} / ${route.name}`" :value="route.id" /></el-select></el-form-item><div class="case-form-grid"><el-form-item label="基线轨迹"><el-select v-model="form.baseline_trace_id" style="width:100%"><el-option v-for="trace in routeTraces" :key="trace.id" :label="`#${trace.id} / ${trace.wavelength_nm}nm`" :value="trace.id" /></el-select></el-form-item><el-form-item label="当前轨迹"><el-select v-model="form.current_trace_id" style="width:100%"><el-option v-for="trace in routeTraces" :key="trace.id" :label="`#${trace.id} / ${trace.wavelength_nm}nm`" :value="trace.id" /></el-select></el-form-item><el-form-item label="距离容差 m"><el-input-number v-model="form.distance_tolerance_m" :min="0.1" :max="1000" style="width:100%" /></el-form-item><el-form-item label="损耗增量阈值 dB"><el-input-number v-model="form.loss_increase_db" :min="0.1" :max="20" :step="0.1" style="width:100%" /></el-form-item></div></el-form><template #footer><el-button @click="createOpen = false">取消</el-button><el-button type="primary" :loading="busy" :disabled="!form.route_id || !form.baseline_trace_id || !form.current_trace_id || form.baseline_trace_id === form.current_trace_id" @click="create">建立案例</el-button></template></el-dialog>
  <ReviewDialog v-model="reviewOpen" mode="case" :case-item="current" :loading="busy" @submit="submitReview" />
</template>

<style scoped>
.case-toolbar{display:flex;align-items:center;justify-content:space-between;gap:20px;margin-bottom:14px}.state-track{display:flex;align-items:center;overflow:auto}.state-track span{display:flex;align-items:center;gap:6px;color:var(--text-muted);font-size:11px;font-weight:700;white-space:nowrap}.state-track span:not(:last-child)::after{content:'';width:24px;height:1px;margin:0 7px;background:var(--line-strong)}.state-track i{width:20px;height:20px;display:grid;place-items:center;border:1px solid var(--line-strong);border-radius:50%;font-style:normal}.trace-pair{display:inline-flex;align-items:center;gap:6px}.uncertainty{display:block;margin-top:2px;color:var(--text-muted)}.conclusion{display:-webkit-box;overflow:hidden;-webkit-line-clamp:2;-webkit-box-orient:vertical}.muted{color:var(--text-muted);font-size:12px}.case-form-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:0 14px}
.batch-bench{margin-bottom:14px;padding:14px 16px;border:1px solid var(--line-strong);border-left:3px solid var(--accent);background:var(--surface-strong)}.bench-head{display:flex;align-items:center;gap:8px;margin-bottom:10px}.bench-head strong{font-size:14px}.bench-count{margin-left:4px;color:var(--text-muted);font-size:12px}.bench-count b{color:var(--accent)}.bench-note{margin-left:6px}.bench-controls{display:flex;align-items:center;flex-wrap:wrap;gap:12px}.bench-field{margin-bottom:0}.bench-field :deep(.el-form-item__label){font-size:12px;font-weight:700;padding-bottom:2px}.bench-hint{color:var(--text-muted);font-size:12px}
.batch-results{margin-top:16px;border:1px solid var(--line);background:var(--surface)}.results-head{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:10px;padding:12px 14px;border-bottom:1px solid var(--line)}.results-summary{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.batch-id{color:var(--text-muted);font-family:"SFMono-Regular",Consolas,monospace;font-size:11px}.results-actions{display:flex;align-items:center;gap:10px}.results-table{border:none}.result-ok{display:inline-flex;align-items:center;gap:4px;color:#17604e;font-weight:700;font-size:12px}.result-fail{display:inline-flex;align-items:center;gap:4px;color:#b53b32;font-weight:700;font-size:12px}.fail-reason{display:inline-flex;align-items:center;gap:6px;color:#b53b32;font-size:12px}.error-code{margin-left:2px}.results-empty{padding:14px}
@media(max-width:700px){.case-toolbar{align-items:stretch;flex-direction:column}.case-form-grid{grid-template-columns:1fr}.results-head{align-items:stretch;flex-direction:column}.results-actions{justify-content:space-between}}
</style>
