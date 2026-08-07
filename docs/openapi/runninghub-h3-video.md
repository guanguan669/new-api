# `minimax_h3` 视频生成调用文档

本文档面向中转站的下游 API 用户。`minimax_h3` 使用 OpenAI Video 兼容协议，调用方不需要注册或调用 RunningHub，也不需要传工作流 ID、上游 API Key 或 `instanceType`。

## 1. 接入信息

| 项目 | 值 |
| --- | --- |
| 中转站地址 | `http://103.36.63.156:3000` |
| 模型名 | `minimax_h3` |
| 鉴权方式 | `Authorization: Bearer <API_KEY>` |
| 创建任务 | `POST /v1/videos` |
| 查询任务 | `GET /v1/videos/{task_id}` |
| 下载视频 | `GET /v1/videos/{task_id}/content` |
| 协议 | OpenAI Video 兼容 |

本文所有可直接复制的示例均使用当前中转站地址：

```bash
export API_KEY="sk-xxxxxxxxxxxxxxxx"
```

请勿把 API Key 放进浏览器前端、公开仓库或截图中。

当前服务地址使用 HTTP。生产环境接入时，建议在该服务前配置自己的 HTTPS 域名；更换域名后，只需把示例中的 `http://103.36.63.156:3000` 改为新域名，接口路径和参数保持不变。

### 1.1 完整调用流程

```text
1. POST /v1/videos 创建异步任务
2. 保存响应中的 id（task_id）
3. GET /v1/videos/{task_id} 轮询状态
4. status=completed 后读取 metadata.url，或调用 /content 下载 MP4
```

`minimax_h3` 根据有没有参考图片自动选择模式：不传图片为文生视频；传入一张或多张图片为图生视频。参考音频可选，并可和上述两种模式组合使用。

### 1.2 中转站开放能力

| 能力 | 下游调用方式 | 是否开放 |
| --- | --- | --- |
| 创建文生/图生视频任务 | `POST /v1/videos` | 是 |
| 图片、音频素材上传 | `POST /v1/videos` 的 multipart 表单，或传公网 URL | 是 |
| 查询进度和结果 | `GET /v1/videos/{task_id}` | 是 |
| 下载生成的视频 | `GET /v1/videos/{task_id}/content` | 是 |
| 直接调用 RunningHub 工作流、节点或工作流 JSON | 不对下游开放 | 否 |
| 直接调用 RunningHub 上传、创建、查询接口 | 不对下游开放 | 否 |
| 取消任务、Webhook 回调 | 当前没有 H3 专用下游接口 | 否 |

中转站会在内部完成工作流校验、媒体上传、任务创建和上游轮询。下游只需使用本页的三个 `/v1/videos` 接口，不要向请求中加入 `workflowId`、`apiKey`、`nodeInfoList`、`workflow` 或 `instanceType` 等 RunningHub 原生字段。

## 2. 最快开始：文生视频

提交任务后接口会立即返回任务 ID；视频生成是异步的，需要继续轮询任务状态。

```bash
curl --request POST "http://103.36.63.156:3000/v1/videos" \
  --header "Authorization: Bearer $API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "minimax_h3",
    "prompt": "一位身穿红色连衣裙的舞者在明亮舞台上自然起舞，镜头平稳推进，电影级光影",
    "seconds": 10,
    "size": "1920x1080"
  }'
```

创建成功的示例响应：

```json
{
  "id": "task_xxxxxxxxxxxxxxxxx",
  "task_id": "task_xxxxxxxxxxxxxxxxx",
  "object": "video",
  "model": "minimax_h3",
  "status": "queued",
  "progress": 0,
  "created_at": 1786089600
}
```

保存 `id`（或兼容字段 `task_id`）用于后续查询。

## 3. 图生视频

只要请求中带有参考图片，中转站会自动选择图生视频工作流。图片可以是公网 `http`/`https` URL，也可以使用 multipart 文件上传。

### 3.1 使用图片 URL

```bash
curl --request POST "http://103.36.63.156:3000/v1/videos" \
  --header "Authorization: Bearer $API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "minimax_h3",
    "prompt": "让画面中的人物随音乐自然跳舞，保持人物外观和服装一致，背景光线柔和",
    "seconds": 10,
    "size": "1920x1080",
    "input_reference": "https://cdn.example.com/person.png"
  }'
```

多张参考图使用 `images` 或 `reference_images` 数组：

```json
{
  "model": "minimax_h3",
  "prompt": "将这些人物和场景元素融合成一段自然舞蹈视频",
  "seconds": 5,
  "size": "1376x768",
  "images": [
    "https://cdn.example.com/person.png",
    "https://cdn.example.com/outfit.png"
  ]
}
```

### 3.2 使用本地文件上传

不要把本地路径写进 JSON。需要上传本地文件时，请使用 `multipart/form-data`：

```bash
curl --request POST "http://103.36.63.156:3000/v1/videos" \
  --header "Authorization: Bearer $API_KEY" \
  --form "model=minimax_h3" \
  --form "prompt=让人物在舞台上自然跳舞，镜头稳定，动作流畅" \
  --form "seconds=10" \
  --form "size=1920x1080" \
  --form "input_reference=@./person.png"
```

可重复传入 `input_reference`、`image`、`images` 或 `reference_images` 文件字段来提交多张图片。重复的同一个 URL 或同一个已上传文件名会自动去重。

## 4. 音频参考

可选地传入参考音频。没有图片时，音频参考仍走文生视频；有图片时会与图生视频参考一起使用。

JSON URL 示例：

```json
{
  "model": "minimax_h3",
  "prompt": "人物根据参考音频的节奏跳舞，动作自然连贯",
  "seconds": 10,
  "size": "1920x1080",
  "reference_audio": "https://cdn.example.com/music.mp3"
}
```

multipart 示例：

```bash
curl --request POST "http://103.36.63.156:3000/v1/videos" \
  --header "Authorization: Bearer $API_KEY" \
  --form "model=minimax_h3" \
  --form "prompt=人物跟随参考节奏跳舞" \
  --form "seconds=10" \
  --form "size=1920x1080" \
  --form "input_reference=@./person.jpg" \
  --form "reference_audio=@./music.mp3"
```

## 5. 请求参数

### 5.0 完整 JSON 请求结构

除 `model` 和 `prompt` 外，其余字段均为可选。以下是该模型支持的完整下游请求结构；只使用实际需要的字段即可。

```json
{
  "model": "minimax_h3",
  "prompt": "必填：描述主体、动作、场景和镜头语言",
  "seconds": 10,
  "duration": 10,
  "size": "1920x1080",
  "input_reference": "https://cdn.example.com/first-image.png",
  "image": "https://cdn.example.com/first-image.png",
  "images": [
    "https://cdn.example.com/first-image.png",
    "https://cdn.example.com/second-image.png"
  ],
  "reference_images": [
    "https://cdn.example.com/third-image.png"
  ],
  "reference_audio": "https://cdn.example.com/music.mp3",
  "reference_audios": [
    "https://cdn.example.com/voice.mp3"
  ],
  "resolution": "1080P",
  "clarity": "2.0",
  "aspect_ratio": "16:9",
  "megapixels": 2.0
}
```

不要同时为同一种设置传多个互相冲突的值。例如，常规调用只传 `size: "1920x1080"`，不再额外传 `resolution`、`clarity`、`aspect_ratio` 或 `megapixels`。`input_reference`、`image`、`images` 和 `reference_images` 是参考图片的兼容字段；相同 URL 会自动去重。

`metadata` 对象也可以承载 `resolution`、`clarity`、`aspect_ratio`、`megapixels`、`reference_images`、`reference_audio` 和 `reference_audios`，但新接入建议使用请求顶层字段，便于排查和日志分析。

### 5.1 基础参数

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 是 | 固定为 `minimax_h3`。 |
| `prompt` | string | 是 | 视频提示词，不能为空。建议描述主体、动作、场景、镜头和风格。 |
| `seconds` | integer/string | 否 | 目标时长，推荐传正整数。`duration` 是等价别名；两者都有时优先用 `duration`。未传时默认按 5 秒处理。 |
| `size` | string | 否 | 画幅和清晰度，推荐使用 `宽x高`，例如 `1920x1080` 或 `1376x768`。 |

建议传入 `1` 到 `3600` 秒的正整数。省略 `seconds`/`duration`（或传入 `0`）时，中转站按 5 秒处理；实际可生成时长仍受当前 H3 工作流和上游资源约束。生产调用建议从 5 秒或 10 秒开始测试。

### 5.2 画幅与清晰度

最简单的传法是 `size`：

| 目标 | 推荐 `size` | 实际选择 |
| --- | --- | --- |
| 基准档横屏 | `1376x768` | `16:9`、1.0 MP |
| 1080P 横屏 | `1920x1080` | `16:9`、2.0 MP、自动使用上游 `plus` 实例 |
| 竖屏 | `720x1280` | `9:16`、自动匹配最近的 H3 像素档 |
| 方形 | `1024x1024` | `1:1`、自动匹配最近的 H3 像素档 |

允许的画幅为：`1:1`、`2:3`、`3:2`、`3:4`、`4:3`、`9:16`、`16:9`、`21:9`。所给尺寸的画幅必须足够接近其中之一；不支持任意比例。

高级调用可直接传 `aspect_ratio` 和 `megapixels`。这两个字段可以放在请求顶层，也可以放在 `metadata` 对象中：

```json
{
  "model": "minimax_h3",
  "prompt": "夜晚城市屋顶上的舞蹈短片",
  "seconds": 10,
  "aspect_ratio": "16:9",
  "megapixels": 2.0
}
```

`megapixels` 仅支持：`0.2`、`0.3`、`0.4`、`0.5`、`0.6`、`0.7`、`0.8`、`0.9`、`0.98`、`1.0`、`1.2`、`1.5`、`1.8`、`2.0`。`resolution` 和 `clarity` 也是兼容别名，可传上述像素档、支持的尺寸，或如 `1080P` 的分辨率标记。

参数覆盖顺序为：默认值 -> `size` -> `resolution` -> `clarity` -> `aspect_ratio` -> `megapixels`。因此同时使用多种清晰度参数时，最后两项会覆盖前面的选择。常规业务只传 `size` 即可。

`instanceType` 不属于下游参数。对于 2.0 MP（1080P 档），中转站会自动传递上游所需的 `plus`；调用方不需要也不应设置它。

H3 的输出画布按 32 像素对齐。因此传入 `1920x1080` 时，实际完成文件可能为 `1920x1088`；这属于模型工作流的正常输出，不是接口错误。

### 5.3 参考素材参数与限制

| 类别 | JSON 字段 | multipart 文件字段 | 上限 |
| --- | --- | --- | --- |
| 参考图片 | `input_reference`、`image`、`images`、`reference_images` | `input_reference`、`image`、`images`、`reference_images` | 9 张 |
| 参考音频 | `reference_audio`、`reference_audios` | `reference_audio`、`reference_audios` | 3 段 |

图片、音频 URL 必须是中转站服务器可访问的公网 `http` 或 `https` 地址。内网、回环、本机和受 SSRF 防护拦截的地址会被拒绝。单个上传或下载的参考文件默认上限为 64 MiB；管理员可通过服务配置调整该上限。

当前 `minimax_h3` 不支持参考视频。请不要传 `reference_video`、`reference_videos`，也不要上传名称中包含 `video` 的文件字段。

一次请求只创建一个视频任务；`n` 和 `count` 不用于批量生成。需要多条视频时，请并发或串行创建多次任务，并分别轮询结果。

### 5.4 素材处理过程

中转站不会把下游提交的图片或音频 URL 原样交给工作流。素材处理过程如下：

1. 对 URL 做安全校验，拒绝内网、回环和不安全的地址。
2. 中转站下载公网 URL，或读取 multipart 上传的文件。
3. 中转站把素材上传到上游，并将得到的内部文件标识写入 H3 工作流。
4. 下游始终只需保留自己的 URL 或原始文件，无需保存或传递上游文件名。

建议使用可稳定访问、有效期覆盖整个生成过程的 HTTPS URL。中转站不对素材做格式转换；请上传能被常见视频工作流解码的标准图片和音频文件。素材下载或上游解析失败时，任务会返回 `failed` 或创建错误。

## 6. 查询任务与取得成片

建议每 3 到 10 秒轮询一次，直到状态为 `completed` 或 `failed`。

```bash
export TASK_ID="task_xxxxxxxxxxxxxxxxx"

curl --request GET "http://103.36.63.156:3000/v1/videos/$TASK_ID" \
  --header "Authorization: Bearer $API_KEY"
```

任务完成时的典型响应：

```json
{
  "id": "task_xxxxxxxxxxxxxxxxx",
  "object": "video",
  "model": "minimax_h3",
  "status": "completed",
  "progress": 100,
  "created_at": 1786089600,
  "completed_at": 1786089720,
  "metadata": {
    "url": "https://.../video.mp4"
  }
}
```

状态含义：

| `status` | 含义 | 客户端处理建议 |
| --- | --- | --- |
| `queued` | 已受理，等待上游执行 | 继续轮询。 |
| `in_progress` | 上游正在生成 | 继续轮询。 |
| `completed` | 已成功完成 | 从 `metadata.url` 读取成片 URL，或调用内容下载接口。 |
| `failed` | 生成失败 | 读取 `error.message`，修正参数或稍后重试。 |

### 6.1 响应字段说明

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 中转站生成的公开任务 ID；后续查询统一使用它。 |
| `task_id` | string | 与 `id` 相同的兼容字段，创建响应中可能出现。 |
| `object` | string | 固定为 `video`。 |
| `model` | string | 固定为 `minimax_h3`。 |
| `status` | string | `queued`、`in_progress`、`completed` 或 `failed`。 |
| `progress` | integer | 进度百分比。排队任务通常为 `0`，成功完成为 `100`。 |
| `created_at` | integer | Unix 秒级时间戳。 |
| `completed_at` | integer | 完成或失败时的 Unix 秒级时间戳。 |
| `metadata.url` | string | 仅任务成功时有意义，指向上游返回的视频结果。 |
| `error.message` | string | 仅 `failed` 时出现，描述失败原因。 |

任务 ID 按中转站账号隔离。查询或下载时应使用创建该任务的同一账号下的 API Key；不要把任务 ID 当作公开的视频链接分发。

### 6.2 错误响应格式

创建、查询或下载失败时，服务返回 JSON 错误对象。客户端应以 HTTP 状态码为主，并记录 `code` 和 `message`：

```json
{
  "code": "invalid_request",
  "message": "prompt is required",
  "data": null
}
```

| HTTP 状态 | 常见含义 | 客户端建议 |
| --- | --- | --- |
| `400` | 参数缺失、尺寸或素材字段不支持 | 修正请求，不要原样重试。 |
| `401` / `403` | API Key 无效、已禁用，或没有模型/分组权限 | 检查 Key、模型权限和余额配置。 |
| `413` | 请求体或上传素材超过服务限制 | 压缩素材或改用更小文件。 |
| `429` | 当前分组或上游负载饱和 | 使用退避重试，不要高频轮询。 |
| `5xx` | 中转站或上游暂时不可用 | 稍后重试；创建成功后不要因为排队而重复提交。 |

`metadata.url` 是成片的直接结果 URL。若希望始终经由中转站读取，或不想在客户端依赖该 URL 的生命周期，可下载内容流：

```bash
curl --location "http://103.36.63.156:3000/v1/videos/$TASK_ID/content" \
  --header "Authorization: Bearer $API_KEY" \
  --output minimax-h3-result.mp4
```

只有任务已完成时，内容下载接口才会返回视频文件。

## 7. JavaScript 轮询示例

```javascript
const baseUrl = 'http://103.36.63.156:3000';
const apiKey = process.env.API_KEY;

const headers = {
  Authorization: `Bearer ${apiKey}`,
  'Content-Type': 'application/json',
};

const createResponse = await fetch(`${baseUrl}/v1/videos`, {
  method: 'POST',
  headers,
  body: JSON.stringify({
    model: 'minimax_h3',
    prompt: '一位舞者在现代舞台上自然跳舞，镜头缓慢推进，光影柔和',
    seconds: 10,
    size: '1920x1080',
  }),
});

if (!createResponse.ok) {
  throw new Error(await createResponse.text());
}

const task = await createResponse.json();

for (;;) {
  await new Promise((resolve) => setTimeout(resolve, 5000));

  const response = await fetch(`${baseUrl}/v1/videos/${task.id}`, {
    headers: { Authorization: `Bearer ${apiKey}` },
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }

  const current = await response.json();
  if (current.status === 'completed') {
    console.log('Video URL:', current.metadata.url);
    break;
  }
  if (current.status === 'failed') {
    throw new Error(current.error?.message || 'Video generation failed');
  }
}
```

### 7.1 Python 轮询示例

```python
import os
import time
import requests

BASE_URL = "http://103.36.63.156:3000"
API_KEY = os.environ["API_KEY"]
HEADERS = {"Authorization": f"Bearer {API_KEY}"}

create = requests.post(
    f"{BASE_URL}/v1/videos",
    headers={**HEADERS, "Content-Type": "application/json"},
    json={
        "model": "minimax_h3",
        "prompt": "一位舞者在现代舞台上自然跳舞，镜头缓慢推进，光影柔和",
        "seconds": 10,
        "size": "1920x1080",
    },
    timeout=90,
)
create.raise_for_status()
task = create.json()
task_id = task["id"]

while True:
    time.sleep(5)
    result = requests.get(
        f"{BASE_URL}/v1/videos/{task_id}", headers=HEADERS, timeout=30
    )
    result.raise_for_status()
    video = result.json()

    if video["status"] == "completed":
        print(video["metadata"]["url"])
        break
    if video["status"] == "failed":
        raise RuntimeError(video.get("error", {}).get("message", "task failed"))
```

### 7.2 超时、重试与幂等性

1. 创建视频是异步操作，但创建请求本身还会执行素材下载、上传和上游任务提交；HTTP 客户端超时建议至少设为 60 秒。
2. 拿到 `id` 后只轮询，不要因为任务长时间处于 `queued` 或 `in_progress` 就再次提交同一个请求。
3. `429`、网络抖动和暂时性 `5xx` 可以采用指数退避，例如 2 秒、4 秒、8 秒后重试。
4. 当前接口没有 `Idempotency-Key` 去重约定。若客户端在创建请求超时后没有拿到任务 ID，直接重发可能创建两条任务并产生两次计费；请先查询中转站任务日志或联系管理员确认。
5. 不要高频轮询。建议正常情况下每 5 秒查询一次，长时间排队可调整为 10 秒。

## 8. 计费说明

对外显示的 H3 标准价格如下：

| 分辨率档位 | 价格 |
| --- | --- |
| **768P** | **0.10 元/秒** |
| **2K / 1080P 档** | **0.30 元/秒** |

图生视频的前 5 张参考图片包含在该任务的基础价格中；从第 6 张开始，每多 1 张额外加收 **0.10 元/张**。参考音频不单独按条计费。

中转站的余额、额度和日志可能以 USD 额度展示，因此扣减数字不一定直接等于人民币金额。系统会按站点设置的汇率将上述人民币价格换算为额度；请以创建任务时的模型价格、用户分组倍率和账单记录为准。

### 8.1 自定义清晰度的计费规则

标准价格表只展示 768P 和 2K 两档。若调用方传入其他受支持的 `megapixels` 档位，中转站按当前 H3 价格规则计算：

```text
基础价格 = 0.10 元/秒（1.0 MP）
1.0 MP：基础价格 × 秒数
大于 1.0 MP：基础价格 × 秒数 × megapixels × 1.5
小于 1.0 MP：基础价格 × 秒数 × megapixels × 1.1
```

例如，10 秒、2.0 MP（`1920x1080` 请求，实际可能输出 `1920x1088`）的基础费用为 `0.10 × 10 × 2.0 × 1.5 = 3.00 元`。如账号所在用户分组配置了额外倍率，最终额度还会叠加该分组倍率。

## 9. 常见错误与排查

| 现象 | 常见原因 | 处理方式 |
| --- | --- | --- |
| `prompt is required` | 提示词为空 | 提供非空 `prompt`。 |
| `unsupported runninghub size` | 尺寸比例不在支持范围内 | 使用支持的画幅，优先传标准 `宽x高`。 |
| `unsupported runninghub megapixels` | 像素档不在可选列表 | 使用第 5.2 节列出的固定数值。 |
| `at most 9 reference images` / `at most 3 reference audio files` | 参考素材超过上限 | 减少图片或音频数量。 |
| URL 下载失败或被拒绝 | URL 不公开、过期、内网地址或文件过大 | 使用稳定的公网 URL，或改为 multipart 上传。 |
| `reference video` 不支持 | 传了视频参考素材 | 当前模型只支持文本、图片和音频参考。 |
| 任务 `failed` | 上游资源、工作流、素材解码或内容校验失败 | 查看 `error.message`；确认素材可用后重新创建任务。 |

## 10. 调用建议

1. 新接入先使用 5 秒、单张图片或纯文本进行联调。
2. 客户端保存任务 ID，并实现轮询超时与失败重试策略；不要把创建接口当作同步视频下载接口。
3. 1080P 请求直接传 `size: "1920x1080"`，中转站会自动处理上游所需实例参数。
4. 使用公网稳定的素材 URL，或用 multipart 上传，避免临时链接在上游下载前失效。
5. 调用方只需使用 `minimax_h3` 原始模型名，不需要配置模型映射。

## 11. 与 RunningHub 原生接口的对应关系

RunningHub 原生文档中的请求通常包含 API Key、Workflow ID、节点输入和实例类型。中转站把这些上游概念隐藏起来，只保留下游真正需要的业务参数。下面的表用于排查“参数有没有传到上游”，不代表下游可以直接把右侧字段放进请求。

| 下游参数/行为 | 中转站内部处理 | H3 工作流中的作用 |
| --- | --- | --- |
| `model: "minimax_h3"` | 固定选择 RunningHub H3 适配器 | 使用 H3 模型；拉取上游模型时只返回这一原始模型名。 |
| `prompt` | 写入提示词输入 | 写入 H3 提示词节点。 |
| `seconds` / `duration` | 统一为整数秒并参与计费 | 写入 H3 时长输入。 |
| `size` / `resolution` / `clarity` | 解析为画幅、MP 档位和 32 像素对齐参数 | 写入 H3 分辨率选择器。 |
| `input_reference` / `images` | URL 下载或 multipart 读取后上传 | 按顺序填入最多 9 个图片参考槽位。 |
| `reference_audio(s)` | URL 下载或 multipart 读取后上传 | 按顺序填入最多 3 个音频参考槽位。 |
| 图片是否存在 | 自动在文生和图生工作流之间选择 | 图片工作流使用参考图路径；无图走文本工作流。 |
| `size: "1920x1080"` 或 `megapixels: 2.0` | 自动添加上游要求的 `instanceType: "plus"` | 选择 H3 的高分辨率实例。 |

当前默认会在工作流执行前校验工作流结构；如果管理员在 RunningHub 画布中删除或改名了必需输入，接口会返回明确的工作流错误，而不是让下游盲目等待一个永远不会完成的任务。

### 11.2 管理员参考：内部工作流和节点映射

以下内容仅用于维护中转站和核对画布，不是下游请求格式。工作流被替换时，应同步更新适配器和工作流校验，不要要求下游用户修改调用参数。

| 用途 | 内部配置/节点 |
| --- | --- |
| 图生视频工作流 | `2085203051589230594` |
| 文生视频工作流 | `2085414751793930242` |
| 提示词 | 节点 `138` 的 `value` |
| 时长 | 节点 `132` 的 `value` |
| 画幅、清晰度、对齐倍数 | 节点 `115` 的 `aspect_ratio`、`megapixels`、`multiple` |
| 参考图片 | 节点 `137`、`618`、`617`、`619`、`627`、`626`、`625`、`624`、`623`，按输入顺序使用 |
| 参考音频 | 节点 `628`、`630`、`629`，按输入顺序使用 |
| 最终视频输出 | 当前工作流要求以 `SaveVideo` 节点 `92` 作为最终视频结果 |

中转站内部对应的 RunningHub 原生接口为：创建任务 `/task/openapi/create`、查询任务 `/openapi/v2/query`、二进制素材上传 `/openapi/v2/media/upload/binary`，以及工作流格式校验 `/api/openapi/getJsonApiFormat`。这些路径只由中转站服务端调用，下游不要直接请求。

### 11.1 下游不要传的上游字段

以下字段属于 RunningHub 原生请求，不是本中转站的公开协议：

```text
apiKey
workflowId
workflow
nodeInfoList
instanceType
taskId
```

它们可能被当作未知字段、被忽略或导致请求校验失败。尤其不要把 RunningHub Key 放在下游请求中；下游只使用中转站分配的 `sk-...` API Key。

## 12. 常见问题

### Q1：可以直接使用 OpenAI SDK 吗？

可以。只要 SDK 支持自定义 `baseURL` 和 `POST /v1/videos`，将 Base URL 指向 `http://103.36.63.156:3000`，请求中的模型名使用 `minimax_h3`。如果 SDK 只支持固定的官方模型列表，请改用 SDK 的原始 HTTP、`fetch`、`requests` 或 `curl` 调用方式。

### Q2：为什么传 1080P 后文件是 1920x1088？

H3 工作流按 32 像素对齐画布。`1920x1080` 会选择 2.0 MP 档位，但实际输出高度可能对齐为 1088；这不表示请求失败，播放器和常见视频处理工具都可以正常读取。

### Q3：图片 URL 必须从浏览器能打开吗？

至少必须让中转站服务器能访问。需要登录、仅在本机或内网可访问的 URL 不适合作为参考素材。遇到 CDN 防盗链时，改用 multipart 文件上传最稳定。

### Q4：任务一直是 `queued`，需要重新提交吗？

不需要。先按 5 到 10 秒间隔继续查询；重复提交会创建新任务，并可能产生重复计费。只有收到明确的 `failed` 或创建接口错误时才按错误信息处理。

### Q5：如何生成 15 秒视频？

在创建请求中传 `"seconds": 15` 或 `"duration": 15`。建议先用 5 秒确认提示词和素材无误，再逐步增加时长；长时长任务的排队和生成时间会明显增加。

### Q6：可以传参考视频或用一条请求拼接多个视频吗？

当前 H3 下游适配只支持文本、图片和音频参考，不支持参考视频，也不会自动把多个任务拼接成一条视频。需要多段视频时，请创建多个任务并在客户端或独立剪辑服务中合成。

## 13. 安全与上线检查

- API Key 使用 `Authorization: Bearer ...` 传输，不要放在 URL 查询参数中。
- 当前示例地址是 HTTP；正式对外服务建议使用 HTTPS 反向代理和域名。
- 参考素材 URL 不要包含长期有效的敏感签名，优先使用短期、最小权限的下载链接。
- 在客户端限制并发数和轮询频率，避免触发 `429`。
- 记录 `task_id`、请求时长、清晰度和最终状态；不要记录完整 API Key 或含隐私的素材 URL。
- 以中转站账单记录为准核对人民币价格和额度换算，不要直接把 USD 额度数字当成人民币金额。

## 14. 参考

- [RunningHub 官方 API 文档](https://www.runninghub.cn/runninghub-api-doc-cn/doc-8287342)
- 本文只描述中转站公开的 OpenAI Video 兼容接口；RunningHub 原生接口的字段、返回码和工作流要求发生变化时，以中转站适配器的实际校验结果为准。
