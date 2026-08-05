<template>
  <UserWorkspaceLayout>
    <div class="video-docs">
      <header class="video-docs__header">
        <div>
          <span><Icon name="book" size="sm" /> 视频 API · v1.1 + Seedance 音频升级</span>
          <h1>视频接口接入文档</h1>
          <p>兼容 OpenAI 风格的 Bearer 鉴权，通过你的平台统一接入上游视频模型。</p>
        </div>
        <router-link to="/videos"><Icon name="play" size="sm" /> 打开视频创作</router-link>
      </header>

      <div class="video-docs__layout">
        <aside class="video-docs__nav" aria-label="文档目录">
          <a v-for="item in navigation" :key="item.id" :href="`#${item.id}`">
            <Icon :name="item.icon" size="sm" />
            <span>{{ item.label }}</span>
          </a>
        </aside>

        <main class="video-docs__content">
          <section id="quickstart" class="video-doc-section">
            <div class="video-doc-section__heading">
              <span>01</span><div><h2>快速开始</h2><p>所有资源都按创建它们的 API Key 隔离。</p></div>
            </div>
            <dl class="video-doc-facts">
              <div><dt>Base URL</dt><dd><code>{{ baseUrl }}</code><CopyButton :value="baseUrl" /></dd></div>
              <div><dt>鉴权</dt><dd><code>Authorization: Bearer YOUR_API_KEY</code></dd></div>
              <div><dt>创建模式</dt><dd><code>Prefer: respond-async</code></dd></div>
              <div><dt>幂等键</dt><dd>1-128 个可打印 ASCII 字符，同一请求重试时必须复用</dd></div>
            </dl>
            <CodeBlock title="查询当前 Key 可用模型" :code="modelsExample" />
            <p class="video-doc-note"><Icon name="infoCircle" size="sm" /> 客户端应始终以 <code>GET /v1/models</code> 实际返回为准，不要假设不同 Key 拥有相同模型。</p>
          </section>

          <section id="generate" class="video-doc-section">
            <div class="video-doc-section__heading">
              <span>02</span><div><h2>创建视频</h2><p>创建成功返回 202，平台异步生成并结算。</p></div>
            </div>
            <div class="video-endpoint"><b>POST</b><code>/v1/videos/generations</code><em>202 Accepted</em></div>
            <CodeBlock title="文生视频" :code="generateExample" />
            <div class="video-doc-table-wrap">
              <table>
                <thead><tr><th>字段</th><th>类型</th><th>必填</th><th>说明</th></tr></thead>
                <tbody>
                  <tr v-for="row in requestFields" :key="row.name"><td><code>{{ row.name }}</code></td><td>{{ row.type }}</td><td>{{ row.required }}</td><td>{{ row.description }}</td></tr>
                </tbody>
              </table>
            </div>
            <p class="video-doc-note video-doc-note--warning"><Icon name="exclamationTriangle" size="sm" /> 创建接口严格校验 JSON。未知字段、Base64、data URL 或直接把媒体字节放入 JSON 都会被拒绝。</p>
          </section>

          <section id="uploads" class="video-doc-section">
            <div class="video-doc-section__heading">
              <span>03</span><div><h2>素材上传</h2><p>先上传，再把返回的绝对 URL 放入创建请求。</p></div>
            </div>
            <div class="video-endpoint"><b>POST</b><code>/v1/videos/uploads</code><em>multipart/form-data</em></div>
            <CodeBlock title="上传首帧" :code="uploadExample" />
            <div class="video-doc-limits">
              <article><Icon name="image" size="md" /><strong>图片</strong><span>PNG / JPEG / WebP</span><small>最大 10 MiB</small></article>
              <article><Icon name="play" size="md" /><strong>视频</strong><span>MP4 / MOV（ISO BMFF）</span><small>最大 100 MiB</small></article>
              <article><Icon name="cloud" size="md" /><strong>音频</strong><span>MP3 / WAV（16/24-bit PCM）</span><small>2-30 秒，最大 15 MiB</small></article>
            </div>
            <CodeBlock title="使用上传结果作为首帧" :code="frameExample" />
            <p class="video-doc-note"><Icon name="infoCircle" size="sm" /> <code>image_url</code> 是旧兼容别名，不能和 <code>start_frame_url</code> 同时使用；新接入统一使用后者。</p>
          </section>

          <section id="references" class="video-doc-section">
            <div class="video-doc-section__heading">
              <span>04</span><div><h2>参考媒体与音轨</h2><p>参考媒体放在 guidances 中，生成音轨使用顶层 audio 开关。</p></div>
            </div>
            <div class="video-audio-compare">
              <div><strong><code>audio: true</code></strong><span>让模型为新视频生成匹配音轨</span><small>Seedance 2.0 / Fast / Mini 已支持，默认 false</small></div>
              <div><strong><code>guidances.audio_reference</code></strong><span>上传一段音频供节奏、动作或风格参考</span><small>Seedance 使用时还需至少一张参考图片或一个参考视频</small></div>
            </div>
            <CodeBlock title="Seedance 参考图片与音频" :code="guidanceExample" />
            <ul class="video-doc-rules">
              <li>Seedance：最多参考图片 4 张、参考视频 3 个、参考音频 1 个；支持首帧和尾帧。</li>
              <li>Happy Horse：最多参考图片 9 张；支持首帧，不支持尾帧。</li>
              <li>Grok Imagine：必须且只能提供一张首帧，不支持尾帧与 guidances。</li>
              <li>LTX：支持首帧、尾帧、生成音轨和提示词增强，不支持 guidances。</li>
              <li>首帧或尾帧不能与 <code>guidances.image_reference</code> 同时使用。</li>
            </ul>
          </section>

          <section id="models" class="video-doc-section">
            <div class="video-doc-section__heading">
              <span>05</span><div><h2>模型能力矩阵</h2><p>平台实际返回的分辨率还会受管理员定价配置约束。</p></div>
            </div>
            <div class="video-doc-table-wrap">
              <table class="video-model-table">
                <thead><tr><th>模型</th><th>分辨率</th><th>时长</th><th>比例</th><th>音轨 / 素材</th></tr></thead>
                <tbody>
                  <tr v-for="row in modelRows" :key="row.model"><td><strong>{{ row.model }}</strong></td><td>{{ row.resolutions }}</td><td>{{ row.duration }}</td><td>{{ row.ratios }}</td><td>{{ row.media }}</td></tr>
                </tbody>
              </table>
            </div>
            <p class="video-doc-note"><Icon name="infoCircle" size="sm" /> Seedance 2.0 的 1080p 最长 12 秒；Seedance 720p 不支持 9:21；Grok 544p / 960p 仅支持 1:1。</p>
          </section>

          <section id="jobs" class="video-doc-section">
            <div class="video-doc-section__heading">
              <span>06</span><div><h2>任务、播放与取消</h2><p>查询、取消和内容下载必须使用创建任务时的同一个 Key。</p></div>
            </div>
            <div class="video-endpoint-list">
              <div><b>GET</b><code>/v1/videos/jobs?limit=20</code><span>最近任务，limit 最大 100</span></div>
              <div><b>GET</b><code>/v1/videos/jobs/{job_id}</code><span>获取并刷新任务</span></div>
              <div><b>DELETE</b><code>/v1/videos/jobs/{job_id}</code><span>取消 pending / running</span></div>
              <div><b>GET</b><code>/v1/videos/jobs/{job_id}/content</code><span>鉴权播放或下载</span></div>
              <div><b>GET</b><code>/v1/videos/uploads/{media_id}/content</code><span>读取已上传素材</span></div>
            </div>
            <CodeBlock title="轮询任务" :code="pollExample" />
            <p class="video-doc-note"><Icon name="clock" size="sm" /> 建议每 5 秒查询一次，长任务逐步退避到 30 秒。状态为 <code>pending</code>、<code>running</code>、<code>settling</code>、<code>completed</code>、<code>failed</code> 或 <code>canceled</code>。</p>
            <p class="video-doc-note video-doc-note--warning"><Icon name="lock" size="sm" /> 浏览器的 <code>&lt;video&gt;</code> 标签不能附加 Bearer 头。前端应先用 fetch 携带 Key 获取 Blob，再把对象 URL 交给播放器；不要把 Key 放进 URL。</p>
          </section>

          <section id="billing" class="video-doc-section">
            <div class="video-doc-section__heading">
              <span>07</span><div><h2>计费与幂等</h2><p>平台负责冻结、捕获或释放余额。</p></div>
            </div>
            <div class="video-billing-flow">
              <div><span>1</span><strong>创建</strong><small>按平台定价预冻结金额</small></div><Icon name="arrowRight" size="sm" />
              <div><span>2</span><strong>生成</strong><small>重复幂等键返回同一任务</small></div><Icon name="arrowRight" size="sm" />
              <div><span>3</span><strong>结算</strong><small>完成扣费，失败或取消释放</small></div>
            </div>
            <ul class="video-doc-rules">
              <li>网络超时或 5xx 后重试同一请求，必须保留原请求体和原 <code>Idempotency-Key</code>。</li>
              <li>不要用同一个幂等键提交不同请求，否则返回 <code>409 IDEMPOTENCY_KEY_CONFLICT</code>。</li>
              <li>响应中的 <code>amount</code> 与 <code>currency</code> 是该任务对下游的计费结果。</li>
              <li>生成记录和扣费记录均归属调用时的 API Key 与用户。</li>
            </ul>
          </section>

          <section id="errors" class="video-doc-section">
            <div class="video-doc-section__heading">
              <span>08</span><div><h2>错误与重试</h2><p>先判断 HTTP 状态，再读取 error.code。</p></div>
            </div>
            <div class="video-doc-table-wrap">
              <table>
                <thead><tr><th>HTTP</th><th>常见错误码</th><th>处理建议</th></tr></thead>
                <tbody>
                  <tr><td>400</td><td>ASYNC_REQUIRED、VIDEO_REQUEST_INVALID、IDEMPOTENCY_KEY_INVALID</td><td>修正请求，不要原样重试</td></tr>
                  <tr><td>401 / 403</td><td>API_KEY_REQUIRED、VIDEO_GENERATION_DISABLED</td><td>检查 Key 与所属视频分组</td></tr>
                  <tr><td>402</td><td>INSUFFICIENT_BALANCE</td><td>充值或降低参数后重新创建</td></tr>
                  <tr><td>404</td><td>VIDEO_RESOURCE_NOT_FOUND</td><td>确认资源由当前 Key 创建</td></tr>
                  <tr><td>409</td><td>IDEMPOTENCY_KEY_CONFLICT、VIDEO_JOB_NOT_CANCELABLE</td><td>更换请求键或刷新状态</td></tr>
                  <tr><td>422</td><td>VIDEO_MODEL_INVALID、VIDEO_RESOLUTION_INVALID、VIDEO_DURATION_INVALID、VIDEO_MEDIA_INVALID</td><td>根据模型矩阵修正参数</td></tr>
                  <tr><td>429</td><td>VIDEO_CAPACITY_EXHAUSTED</td><td>按 Retry-After 延迟重试</td></tr>
                  <tr><td>503</td><td>VIDEO_EXECUTION_DISABLED、VIDEO_PRICING_UNAVAILABLE、VIDEO_UPSTREAM_UNAVAILABLE</td><td>保留请求与幂等键，稍后重试</td></tr>
                </tbody>
              </table>
            </div>
            <p class="video-doc-note"><Icon name="shield" size="sm" /> 排查问题时提供 <code>X-Request-Id</code>、HTTP 状态、错误码和发生时间，不要发送完整 API Key。</p>
          </section>
        </main>
      </div>
    </div>
  </UserWorkspaceLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, ref } from 'vue'
import UserWorkspaceLayout from '@/components/layout/UserWorkspaceLayout.vue'
import Icon from '@/components/icons/Icon.vue'

const copiedValue = ref('')
const baseUrl = computed(() => typeof window === 'undefined' ? 'https://your-domain.example' : window.location.origin)
const navigation = [
  { id: 'quickstart', label: '快速开始', icon: 'bolt' },
  { id: 'generate', label: '创建视频', icon: 'play' },
  { id: 'uploads', label: '素材上传', icon: 'upload' },
  { id: 'references', label: '参考与音轨', icon: 'image' },
  { id: 'models', label: '模型矩阵', icon: 'grid' },
  { id: 'jobs', label: '任务管理', icon: 'clock' },
  { id: 'billing', label: '计费与幂等', icon: 'dollar' },
  { id: 'errors', label: '错误重试', icon: 'shield' },
] as const

async function copyText(value: string) {
  await navigator.clipboard.writeText(value)
  copiedValue.value = value
  window.setTimeout(() => { if (copiedValue.value === value) copiedValue.value = '' }, 1400)
}

const CopyButton = defineComponent({
  props: { value: { type: String, required: true } },
  setup(props) {
    return () => h('button', {
      type: 'button', class: 'video-copy-button', title: copiedValue.value === props.value ? '已复制' : '复制',
      onClick: () => copyText(props.value),
    }, [h(Icon, { name: copiedValue.value === props.value ? 'check' : 'copy', size: 'sm' })])
  },
})

const CodeBlock = defineComponent({
  props: { title: { type: String, required: true }, code: { type: String, required: true } },
  setup(props) {
    return () => h('div', { class: 'video-code-block' }, [
      h('div', [h('span', props.title), h(CopyButton, { value: props.code })]),
      h('pre', [h('code', props.code)]),
    ])
  },
})

const modelsExample = computed(() => `curl "${baseUrl.value}/v1/models" \\
  -H "Authorization: Bearer YOUR_API_KEY"`)
const generateExample = computed(() => `curl -X POST "${baseUrl.value}/v1/videos/generations" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Prefer: respond-async" \\
  -H "Idempotency-Key: video-order-0001" \\
  -d '{
    "model": "seedance-2.0",
    "prompt": "云海之上的仙侠山门，镜头缓慢向前推进",
    "resolution": "480p",
    "duration": 5,
    "aspect_ratio": "16:9",
    "audio": true
  }'`)
const uploadExample = computed(() => `curl -X POST "${baseUrl.value}/v1/videos/uploads" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -F "file=@opening.webp"`)
const frameExample = computed(() => `{
  "model": "seedance-2.0",
  "prompt": "保持人物一致，衣袂随风，镜头平稳环绕",
  "resolution": "720p",
  "duration": 8,
  "aspect_ratio": "16:9",
  "start_frame_url": "${baseUrl.value}/v1/videos/uploads/vidmedia_xxx/content"
}`)
const guidanceExample = computed(() => `{
  "model": "seedance-2.0",
  "prompt": "根据参考人物与鼓点生成连贯动作",
  "resolution": "720p",
  "duration": 8,
  "aspect_ratio": "16:9",
  "audio": true,
  "guidances": {
    "image_reference": [{
      "image": { "url": "${baseUrl.value}/v1/videos/uploads/vidmedia_image/content", "type": "UPLOADED" },
      "strength": "MID",
      "order": 1
    }],
    "audio_reference": [{
      "audio": { "url": "${baseUrl.value}/v1/videos/uploads/vidmedia_audio/content", "type": "UPLOADED" }
    }]
  }
}`)
const pollExample = computed(() => `curl "${baseUrl.value}/v1/videos/jobs/vidjob_xxx" \\
  -H "Authorization: Bearer YOUR_API_KEY"

curl -L "${baseUrl.value}/v1/videos/jobs/vidjob_xxx/content?download=1" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -o result.mp4`)

const requestFields = [
  { name: 'model', type: 'string', required: '是', description: 'GET /v1/models 返回的模型标识' },
  { name: 'prompt', type: 'string', required: '是', description: '主体、动作、场景、镜头和风格描述' },
  { name: 'resolution', type: 'string', required: '否', description: '省略时使用模型默认分辨率' },
  { name: 'duration', type: 'integer', required: '否', description: '整数秒，受模型和分辨率约束' },
  { name: 'aspect_ratio', type: 'string', required: '否', description: '省略时使用模型默认比例' },
  { name: 'audio', type: 'boolean', required: '否', description: '是否生成音轨，默认 false' },
  { name: 'prompt_enhance', type: 'string', required: '否', description: 'AUTO、ON 或 OFF' },
  { name: 'start_frame_url', type: 'string', required: '否', description: '首帧绝对 HTTP(S) URL' },
  { name: 'end_frame_url', type: 'string', required: '否', description: '尾帧绝对 HTTP(S) URL' },
  { name: 'guidances', type: 'object', required: '否', description: '参考图片、参考视频和参考音频' },
]

const modelRows = [
  { model: 'seedance-2.0', resolutions: '480p / 720p / 1080p', duration: '4-15 秒', ratios: '16:9、9:16、1:1、4:3、3:4、21:9、9:21', media: '音轨、首尾帧、图片/视频/音频参考' },
  { model: 'seedance-2.0-fast', resolutions: '480p / 720p', duration: '4-15 秒', ratios: '7 种 Seedance 比例', media: '音轨、首尾帧、图片/视频/音频参考' },
  { model: 'seedance-2.0-mini', resolutions: '480p / 720p', duration: '4-15 秒', ratios: '16:9、1:1、9:16', media: '音轨、首尾帧、图片/视频/音频参考' },
  { model: 'happy-horse-1.1', resolutions: '720p / 1080p', duration: '3-15 秒', ratios: '16:9、4:3、1:1、3:4、9:16', media: '音轨、首帧、图片参考、提示增强' },
  { model: 'grok-imagine-1.5', resolutions: '400p / 544p / 720p / 960p', duration: '3-15 秒', ratios: '16:9、9:16 或 1:1', media: '音轨、必须首帧' },
  { model: 'ltx-2.3-pro', resolutions: '1080p / 1440p / 2160p', duration: '6 / 8 / 10 秒', ratios: '仅 16:9', media: '音轨、首尾帧、提示增强' },
  { model: 'ltx-2.3-fast', resolutions: '1080p / 1440p / 2160p', duration: '6-20 秒偶数', ratios: '仅 16:9', media: '音轨、首尾帧、提示增强' },
]
</script>

<style scoped>
.video-docs { display: grid; gap: 1rem; color: #211f1c; }
.video-docs__header { display: flex; align-items: end; justify-content: space-between; gap: 1.5rem; border-bottom: 2px solid #211f1c; padding: .4rem .15rem 1rem; }
.video-docs__header span { display: inline-flex; align-items: center; gap: .4rem; color: #08799a; font-size: .7rem; font-weight: 900; }.video-docs__header h1 { margin: .32rem 0 .18rem; font-family: Georgia, 'Songti SC', serif; font-size: 2rem; }.video-docs__header p { margin: 0; color: #716960; font-size: .82rem; }.video-docs__header > a { display: inline-flex; min-height: 2.55rem; align-items: center; gap: .45rem; border: 1px solid #211f1c; border-radius: 6px; background: #ff5f8f; box-shadow: 3px 3px 0 #211f1c; padding: 0 .85rem; color: inherit; font-size: .72rem; font-weight: 900; text-decoration: none; }
.video-docs__layout { display: grid; grid-template-columns: 12rem minmax(0,1fr); align-items: start; gap: 1.5rem; }.video-docs__nav { position: sticky; top: 1rem; display: grid; gap: .2rem; border-left: 2px solid #d9d1c8; padding-left: .55rem; }.video-docs__nav a { display: flex; min-height: 2.35rem; align-items: center; gap: .45rem; border-radius: 5px; padding: 0 .55rem; color: #665f58; font-size: .7rem; font-weight: 800; text-decoration: none; }.video-docs__nav a:hover { background: #fff; color: #08799a; }
.video-docs__content { display: grid; gap: 1rem; min-width: 0; }.video-doc-section { scroll-margin-top: 1rem; border: 1px solid rgba(33,31,28,.2); border-radius: 8px; background: rgba(255,253,245,.95); box-shadow: 0 5px 18px rgba(58,46,36,.06); padding: 1.15rem; }.video-doc-section__heading { display: grid; grid-template-columns: 2.3rem minmax(0,1fr); gap: .7rem; margin-bottom: .9rem; }.video-doc-section__heading > span { display: grid; width: 2.3rem; height: 2.3rem; place-items: center; border: 1px solid #211f1c; border-radius: 6px; background: #ffd447; font-size: .67rem; font-weight: 900; }.video-doc-section h2 { margin: 0; font-family: Georgia, 'Songti SC', serif; font-size: 1.1rem; }.video-doc-section__heading p { margin: .15rem 0 0; color: #756d64; font-size: .68rem; }
.video-doc-facts { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: .45rem; margin: 0 0 .85rem; }.video-doc-facts > div { min-width: 0; border-left: 3px solid #08a9d6; background: #f4fbfd; padding: .55rem .65rem; }.video-doc-facts dt { color: #766e65; font-size: .61rem; font-weight: 800; }.video-doc-facts dd { display: flex; align-items: center; justify-content: space-between; gap: .35rem; min-width: 0; margin: .18rem 0 0; font-size: .67rem; font-weight: 700; }.video-doc-facts code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.video-copy-button { display: grid; flex: 0 0 auto; width: 1.9rem; height: 1.9rem; cursor: pointer; place-items: center; border: 1px solid #d0c6bc; border-radius: 4px; background: #fff; color: #59524b; }
.video-code-block { overflow: hidden; margin-top: .7rem; border: 1px solid #292824; border-radius: 7px; background: #20231f; color: #eef7e8; }.video-code-block > div { display: flex; min-height: 2.2rem; align-items: center; justify-content: space-between; border-bottom: 1px solid #3b4038; padding: 0 .55rem .0 .75rem; color: #bdc8b7; font-size: .62rem; font-weight: 800; }.video-code-block .video-copy-button { border-color: #4a5146; background: #2b3029; color: #dce8d7; }.video-code-block pre { max-height: 25rem; overflow: auto; margin: 0; padding: .85rem; }.video-code-block code { font-family: 'Cascadia Code', Consolas, monospace; font-size: .68rem; line-height: 1.65; white-space: pre; }
.video-doc-note { display: flex; align-items: start; gap: .42rem; margin: .7rem 0 0; border-left: 3px solid #08a9d6; background: #eefbff; padding: .55rem .65rem; color: #3f5a61; font-size: .66rem; line-height: 1.55; }.video-doc-note svg { flex: 0 0 auto; margin-top: .1rem; }.video-doc-note--warning { border-color: #d9a316; background: #fff8df; color: #725a19; }
.video-endpoint { display: flex; min-height: 2.7rem; align-items: center; gap: .6rem; border-block: 1px solid #ddd5cb; padding: .4rem 0; }.video-endpoint b,.video-endpoint-list b { border-radius: 4px; background: #e1f8ff; padding: .25rem .4rem; color: #08799a; font-size: .59rem; }.video-endpoint code { font-size: .72rem; font-weight: 800; }.video-endpoint em { margin-left: auto; color: #747068; font-size: .62rem; font-style: normal; }
.video-doc-table-wrap { overflow-x: auto; margin-top: .7rem; }.video-doc-table-wrap table { width: 100%; border-collapse: collapse; font-size: .65rem; }.video-doc-table-wrap th { border-bottom: 2px solid #bcb1a6; background: #f1ede7; padding: .55rem; color: #625a52; text-align: left; white-space: nowrap; }.video-doc-table-wrap td { border-bottom: 1px solid #e3dcd4; padding: .58rem .55rem; vertical-align: top; line-height: 1.5; }.video-doc-table-wrap code { font-size: .63rem; }.video-model-table td:first-child { white-space: nowrap; }
.video-doc-limits { display: grid; grid-template-columns: repeat(3,minmax(0,1fr)); gap: .55rem; margin-top: .7rem; }.video-doc-limits article { display: grid; grid-template-columns: 2rem minmax(0,1fr); align-items: center; column-gap: .5rem; border: 1px solid #d5ccc3; border-radius: 6px; background: #fff; padding: .65rem; }.video-doc-limits svg { grid-row: 1 / 4; color: #08799a; }.video-doc-limits strong { font-size: .7rem; }.video-doc-limits span,.video-doc-limits small { color: #776f66; font-size: .59rem; }
.video-audio-compare { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: .55rem; }.video-audio-compare > div { display: grid; gap: .25rem; border: 1px solid #d1c7bc; border-radius: 6px; background: #fff; padding: .7rem; }.video-audio-compare strong { color: #08799a; font-size: .72rem; }.video-audio-compare span { font-size: .68rem; font-weight: 800; }.video-audio-compare small { color: #736b63; font-size: .61rem; line-height: 1.45; }
.video-doc-rules { display: grid; gap: .42rem; margin: .75rem 0 0; padding: 0; list-style: none; }.video-doc-rules li { position: relative; padding-left: 1rem; color: #5d564f; font-size: .67rem; line-height: 1.55; }.video-doc-rules li::before { position: absolute; top: .48rem; left: .1rem; width: .35rem; height: .35rem; border-radius: 50%; background: #ff5f8f; content: ''; }
.video-endpoint-list { display: grid; gap: .32rem; }.video-endpoint-list > div { display: grid; grid-template-columns: 3.4rem minmax(15rem,.9fr) minmax(10rem,1fr); align-items: center; gap: .5rem; border-bottom: 1px solid #e1dad2; padding: .45rem 0; }.video-endpoint-list code { font-size: .65rem; font-weight: 800; }.video-endpoint-list span { color: #746c64; font-size: .62rem; }
.video-billing-flow { display: grid; grid-template-columns: 1fr auto 1fr auto 1fr; align-items: center; gap: .5rem; }.video-billing-flow > div { display: grid; grid-template-columns: 1.7rem minmax(0,1fr); align-items: center; column-gap: .45rem; border: 1px solid #d5cbc1; border-radius: 6px; background: #fff; padding: .65rem; }.video-billing-flow div > span { display: grid; grid-row: 1 / 3; width: 1.7rem; height: 1.7rem; place-items: center; border-radius: 50%; background: #ffd447; font-size: .62rem; font-weight: 900; }.video-billing-flow strong { font-size: .69rem; }.video-billing-flow small { color: #776f67; font-size: .59rem; }
@media (max-width: 800px) { .video-docs__header { align-items: stretch; flex-direction: column; }.video-docs__header > a { justify-content: center; align-self: start; }.video-docs__layout { grid-template-columns: 1fr; }.video-docs__nav { position: static; grid-template-columns: repeat(2,minmax(0,1fr)); border-left: 0; padding: 0; }.video-doc-facts,.video-doc-limits,.video-audio-compare { grid-template-columns: 1fr; }.video-endpoint-list > div { grid-template-columns: 3.2rem minmax(0,1fr); }.video-endpoint-list span { grid-column: 2; }.video-billing-flow { grid-template-columns: 1fr; }.video-billing-flow > svg { margin: 0 auto; transform: rotate(90deg); }.video-docs__header h1 { font-size: 1.55rem; } }
</style>
