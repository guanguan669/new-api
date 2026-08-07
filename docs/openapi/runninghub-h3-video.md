# `minimax_h3` 视频生成调用文档

本文档面向中转站的下游 API 用户。`minimax_h3` 使用 OpenAI Video 兼容协议，调用方不需要注册或调用 RunningHub，也不需要传工作流 ID、上游 API Key 或 `instanceType`。

## 1. 接入信息

| 项目 | 值 |
| --- | --- |
| 模型名 | `minimax_h3` |
| 鉴权方式 | `Authorization: Bearer <API_KEY>` |
| 创建任务 | `POST /v1/videos` |
| 查询任务 | `GET /v1/videos/{task_id}` |
| 下载视频 | `GET /v1/videos/{task_id}/content` |
| 协议 | OpenAI Video 兼容 |

将下文的 `BASE_URL` 替换为中转站为你开通的接口地址，例如：

```bash
export BASE_URL="https://api.example.com"
export API_KEY="sk-xxxxxxxxxxxxxxxx"
```

请勿把 API Key 放进浏览器前端、公开仓库或截图中。

## 2. 最快开始：文生视频

提交任务后接口会立即返回任务 ID；视频生成是异步的，需要继续轮询任务状态。

```bash
curl --request POST "$BASE_URL/v1/videos" \
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
curl --request POST "$BASE_URL/v1/videos" \
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
curl --request POST "$BASE_URL/v1/videos" \
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
curl --request POST "$BASE_URL/v1/videos" \
  --header "Authorization: Bearer $API_KEY" \
  --form "model=minimax_h3" \
  --form "prompt=人物跟随参考节奏跳舞" \
  --form "seconds=10" \
  --form "size=1920x1080" \
  --form "input_reference=@./person.jpg" \
  --form "reference_audio=@./music.mp3"
```

## 5. 请求参数

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

## 6. 查询任务与取得成片

建议每 3 到 10 秒轮询一次，直到状态为 `completed` 或 `failed`。

```bash
export TASK_ID="task_xxxxxxxxxxxxxxxxx"

curl --request GET "$BASE_URL/v1/videos/$TASK_ID" \
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

`metadata.url` 是成片的直接结果 URL。若希望始终经由中转站读取，或不想在客户端依赖该 URL 的生命周期，可下载内容流：

```bash
curl --location "$BASE_URL/v1/videos/$TASK_ID/content" \
  --header "Authorization: Bearer $API_KEY" \
  --output minimax-h3-result.mp4
```

只有任务已完成时，内容下载接口才会返回视频文件。

## 7. JavaScript 轮询示例

```javascript
const baseUrl = process.env.BASE_URL;
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

## 8. 计费说明

对外显示的 H3 标准价格如下：

| 分辨率档位 | 价格 |
| --- | --- |
| **768P** | **0.10 元/秒** |
| **2K / 1080P 档** | **0.30 元/秒** |

图生视频的前 5 张参考图片包含在该任务的基础价格中；从第 6 张开始，每多 1 张额外加收 **0.10 元/张**。参考音频不单独按条计费。

中转站的余额、额度和日志可能以 USD 额度展示，因此扣减数字不一定直接等于人民币金额。系统会按站点设置的汇率将上述人民币价格换算为额度；请以创建任务时的模型价格、用户分组倍率和账单记录为准。

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
