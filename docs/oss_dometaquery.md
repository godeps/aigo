调用DoMetaQuery接口查询满足指定条件的文件（Object），并按照指定字段和排序方式列出文件信息。您也可以通过Query的嵌套使用实现复杂查询，以及通过聚合操作实现对不同字段的值进行统计和分析。

## 注意事项

要查询满足指定条件的文件，您必须有`oss:DoMetaQuery`权限。具体操作，请参见[为RAM用户授予自定义的权限策略](https://help.aliyun.com/zh/oss/common-examples-of-ram-policies#section-ucu-jv0-zip)。

## 请求语法

### **标量检索**

```
POST /?metaQuery&comp=query&mode=basic HTTP/1.1
Host: BucketName.oss-cn-hangzhou.aliyuncs.com
Date: GMT Date
Authorization: SignatureValue 
<?xml version="1.0" encoding="UTF-8"?>
<MetaQuery>
  <NextToken></NextToken>
  <MaxResults>5</MaxResults>
  <Query>{"Field": "Size","Value": "1048576","Operation": "gt"}</Query>
  <Sort>Size</Sort>
  <Order>asc</Order>
  <Aggregations>
    <Aggregation>
      <Field>Size</Field>
      <Operation>sum</Operation>
    </Aggregation>
    <Aggregation>
      <Field>Size</Field>
      <Operation>max</Operation>
    </Aggregation>
  </Aggregations>
</MetaQuery>
```

### **向量检索**

```
POST /?metaQuery&comp=query&mode=semantic HTTP/1.1
Host: BucketName.oss-cn-hangzhou.aliyuncs.com
Date: GMT Date
Authorization: SignatureValue 
<?xml version="1.0" encoding="UTF-8"?>
<MetaQuery>
  <MaxResults>99</MaxResults>
  <Query>俯瞰白雪覆盖的森林</Query>
  <MediaTypes>
    <MediaType>image</MediaType>
  </MediaTypes>  
  <SimpleQuery>{"Operation":"gt", "Field": "Size", "Value": "30"}</SimpleQuery>
</MetaQuery>
```

## 请求头

此接口仅涉及公共请求头。更多信息，请参见[公共请求头（Common Request Headers）](https://help.aliyun.com/zh/oss/developer-reference/common-http-headers#section-eq1-b2y-wdb)。

## 请求元素

### **标量检索**

| **名称** | **类型** | **是否必选** | **示例值** | **描述** |
| --- | --- | --- | --- | --- |
| mode | 字符串 | 是   | basic | 指定检索模式为标量检索。 |
| MetaQuery | 容器  | 是   | 不涉及 | 查询条件的容器。 子节点：NextToken、MaxResults、Query、Sort、Order、Aggregations |
| NextToken | 字符串 | 否   | MTIzNDU2Nzg6aW1tdGVzdDpleGFtcGxlYnVja2V0OmRhdGFzZXQwMDE6b3NzOi8vZXhhbXBsZWJ1Y2tldC9zYW1wbGVvYmplY3QxLmpw\\*\\*\\*\\* | 当Object总数大于设置的MaxResults时，用于翻页的token。 从NextToken开始按字典序返回Object信息列表。 第一次调用此接口时，设置此字段为空。 父节点：MetaQuery |
| MaxResults | 整型  | 否   | 5   | 返回Object的最大个数，取值范围为0~100。 不设置此参数或者设置为0时，则默认值为100。 父节点：MetaQuery |
| Query | 字符串 | 是   | {"Field": "Size","Value": "1048576","Operation": "gt"} | 查询条件。包括如下选项： - Operation：操作符。取值范围为eq（等于）、gt（大于）、gte（大于等于）、lt（小于）、 lte（小于等于）、match（模糊查询）、prefix（前缀查询）、and（逻辑与）、or（逻辑或）和not（逻辑非）。 - Field：字段名称。支持的字段以及字段对操作符的支持情况请参见[附录：标量检索的字段和操作符列表](https://help.aliyun.com/zh/oss/developer-reference/appendix-supported-fields-and-operators#concept-2084036)。 - Value：字段值。 - SubQueries：子查询条件，包括的选项与简单查询条件相同。只有当Operations为逻辑运算符（and、or和not）时，才需要设置子查询条件。 关于Query示例的更多信息，请参见[DoMetaQuery](#)。 父节点：MetaQuery |
| Sort | 字符串 | 否   | Size | 对指定字段排序。关于支持排序的字段列表，请参见[附录：标量检索的字段和操作符列表](https://help.aliyun.com/zh/oss/developer-reference/appendix-supported-fields-and-operators#concept-2084036)。 父节点：MetaQuery |
| Order | 字符串 | 否   | asc | 排序方式。取值范围如下： - asc：升序 - desc（默认值）：降序 父节点：MetaQuery |
| Aggregations | 容器  | 否   | 不涉及 | 聚合操作信息的容器。 子节点：Aggregation 父节点：MetaQuery |
| Aggregation | 容器  | 否   | 不涉及 | 单个聚合操作信息的容器。 子节点：Field、Operation 父节点：Aggregations |
| Field | 字符串 | 否   | Size | 字段名称。关于支持的字段以及字段对操作符的支持情况，请参见[附录：标量检索的字段和操作符列表](https://help.aliyun.com/zh/oss/developer-reference/appendix-supported-fields-and-operators#concept-2084036)。 父节点：Aggregation |
| Operation | 字符串 | 否   | sum | 聚合操作中的操作符。取值范围如下： - min：最小值 - max：最大值 - average：平均数 - sum：求和 - count：计数 - distinct：去重统计 - group：分组计数 父节点：Aggregation |

### **向量检索**

| **名称** | **类型** | **是否必选** | **示例值** | **描述** |
| --- | --- | --- | --- | --- |
| mode | 字符串 | 是   | semantic | 指定检索模式为向量检索。 |
| MetaQuery | 容器  | 是   | 不涉及 | 查询条件的容器。 子节点：MaxResults、Query、MediaTypes、SimpleQuery |
| MaxResults | 整型  | 否   | 5   | 返回Object的最大个数，取值范围为0~100。 不设置此参数或者设置为0时，则默认值为100。 父节点：MetaQuery |
| Query | 字符串 | 是   | 俯瞰白雪覆盖的森林 | 输入检索内容。 父节点：MetaQuery |
| MediaTypes | 容器  | 是   | 不涉及 | 多媒体元数据检索条件。 父节点：MetaQuery |
| MediaType | 字符串 | 是   | image | 选择检索的多媒体类型。取值如下： - image：图片 - video：视频 - audio：音频 - document：文档 父节点：MediaTypes |
| SimpleQuery | 字符串 | 否   | {"Operation":"gt", "Field": "Size", "Value": "30"} | 查询条件。包括如下选项： - Operation：操作符。取值范围为eq（等于）、gt（大于）、gte（大于等于）、lt（小于）、 lte（小于等于）、match（模糊查询）、prefix（前缀查询）、and（逻辑与）、or（逻辑或）和not（逻辑非）。 - Field：字段名称。支持的字段以及字段对操作符的支持情况请参见[附录：向量检索的字段和操作符列表](https://help.aliyun.com/zh/oss/developer-reference/appendix-list-of-fields-and-operators-for-vector-retrieval)。 - Value：字段值。 - SubQueries：子查询条件，包括的选项与简单查询条件相同。只有当Operations为逻辑运算符（and、or和not）时，才需要设置子查询条件。 父节点：MetaQuery |

## 响应头

此接口仅涉及公共响应头。更多信息，请参见[公共响应头（Common Response Headers）](https://help.aliyun.com/zh/oss/developer-reference/common-http-headers#section-hcd-m2y-wdb)。

## 响应元素

### **标量检索**

| **名称** | **类型** | **示例值** | **描述** |
| --- | --- | --- | --- |
| MetaQuery | 容器  | 不涉及 | 查询结果的容器。 子节点：NextToken、Files、Aggregations |
| NextToken | 字符串 | MTIzNDU2Nzg6aW1tdGVzdDpleGFtcGxlYnVja2V0OmRhdGFzZXQwMDE6b3NzOi8vZXhhbXBsZWJ1Y2tldC9zYW1wbGVvYmplY3QxLmpw\\*\\*\\*\\* | 当Object总数大于设置的MaxResults时，用于翻页的token。 下一次列出Object信息时以此值为NextToken，将未返回的结果返回。 当Object未全部返回时，此参数才有值。 父节点：MetaQuery |
| Files | 容器  | 不涉及 | Object信息的容器。 子节点：File 父节点：MetaQuery |
| File | 容器  | 不涉及 | 单个Object信息的容器。 子节点：Filename、Size、FileModifiedTime、OSSObjectType、OSSStorageClass、ObjectACL、ETag、OSSTaggingCount、OSSTagging、OSSUserMeta、OSSCRC64、ServerSideEncryption 父节点：Files |
| Filename | 字符串 | exampleobject.txt | Object完整路径。 父节点：File |
| Size | 整型  | 120 | Object大小。单位为字节。 父节点：File |
| FileModifiedTime | 字符串 | 2025-05-19T16:14:38+08:00 | Object的最近一次修改时间，遵循RFC3339Nano标准格式。 父节点：File |
| OSSObjectType | 字符串 | Normal | Object的类型。取值范围如下： - Normal：通过PutObject接口上传的Object类型。 - Appendable：通过AppendObject接口上传的Object类型。 - Multipart：通过MultipartUpload接口上传的Object类型。 - Symlink：通过PutSymlink接口创建的软链接。 父节点：File |
| OSSStorageClass | 字符串 | Standard | Object的存储类型。取值范围如下： - Standard：标准存储类型提供高可靠、高可用、高性能的对象存储服务，能够支持频繁的数据访问。 - IA：低频访问存储类型适合需要长期存储但不经常被访问的数据（平均每月访问频率1到2次）。 - Archive：归档存储类型适合需要长期存储（建议半年以上）的归档数据，在存储周期内极少被访问，数据进入到可读取状态需要1分钟的解冻时间。 - ColdArchive：冷归档存储类型适用于需要长期保存且几乎不访问的数据。 父节点：File |
| ObjectACL | 字符串 | default | Object的访问权限。取值范围如下： - default：Object遵循所在存储空间的访问权限。 - private：Object是私有资源。只有Object的拥有者和授权用户有该Object的读写权限，其他用户没有权限操作该Object。 - public-read：Object是公共读资源。只有Object的拥有者和授权用户有该Object的读写权限，其他用户只有该Object的读权限。请谨慎使用该权限。 - public-read-write：Object是公共读写资源。所有用户都有该Object的读写权限。请谨慎使用该权限。 父节点：File |
| ETag | 字符串 | "fba9dede5f27731c9771645a3986\\*\\*\\*\\*" | Object生成时会创建相应的ETag ，ETag用于标识一个Object的内容。 - 对于PutObject请求创建的Object，ETag值是其内容的MD5值。 - 对于其他方式创建的Object，ETag值是基于一定计算规则生成的唯一值，但不是其内容的MD5值。 **说明** ETag值可以用于检查Object内容是否发生变化。不建议使用ETag作为Object内容的MD5来校验数据完整性。 父节点：File |
| OSSTaggingCount | 整型  | 2   | Object的标签个数。 父节点：File |
| OSSTagging | 容器  | 不涉及 | 标签信息的容器。 子节点：Tagging 父节点：File |
| Tagging | 容器  | 不涉及 | 单个标签信息的容器。 子节点：Key、Value 父节点：OSSTagging |
| Key | 字符串 | owner | 标签或者用户自定义元数据的Key。 用户自定义元数据必须以`x-oss-meta-`为前缀。 父节点：Tagging、UserMeta |
| Value | 字符串 | John | 标签或者用户自定义元数据的Value。 父节点：Tagging、UserMeta |
| OSSUserMeta | 容器  | 不涉及 | 用户自定义元数据的容器。 子节点：UserMeta 父节点：File |
| UserMeta | 容器  | 不涉及 | 单个用户自定义元数据的容器。 子节点：Key、Value 父节点：OSSUserMeta |
| OSSCRC64 | 字符串 | 4858A48BD1466884 | Object的64位CRC值。该64位CRC根据CRC-64/XZ标准计算得出。 父节点：File |
| ServerSideEncryption | 字符串 | AES256 | OSS创建文件时的服务器端加密编码算法。取值范围为AES256。 父节点：File |
| ServerSideEncryptionCustomerAlgorithm | 字符串 | SM4 | 用户在本地客户端加密文件时使用的加密编码算法。 父节点：File |
| Aggregations | 容器  | 不涉及 | 聚合操作信息的容器。 子节点：Field、Operation、Operation、Value、Groups 父节点：MetaQuery |
| Field | 字符串 | Size | 字段名称。 父节点：Aggregations |
| Operation | 字符串 | sum | 聚合操作符。 父节点：Aggregations |
| Value | 浮点数 | 200 | 聚合操作的结果值。 父节点：Aggregations |
| Groups | 容器  | 不涉及 | 分组聚合的结果列表。 子节点：Value、Count 父节点：Aggregations |
| Value | 字符串 | 100 | 分组聚合的值。 父节点：Groups |
| Count | 整型  | 5   | 分组聚合的总个数。 父节点：Groups |

### **向量检索**

| **名称** | **类型** | **示例值** | **描述** |
| --- | --- | --- | --- |
| MetaQuery | 容器  | 不涉及 | 查询结果的容器。 子节点：Files |
| Files | 容器  | 不涉及 | Object信息列表。 子节点：File 父节点：MetaQuery |
| File | 容器  | 不涉及 | 单个Object信息。 父节点：Files |
| URI | 字符串 | oss://examplebucket/test-object.jpg | Object完整路径。 父节点：File |
| Filename | 字符串 | exampleobject.txt | Object名称。 父节点：File |
| Size | 整型  | 120 | Object大小。单位为字节。 父节点：File |
| ObjectACL | 字符串 | default | Object的访问权限。取值范围如下： - default：Object遵循所在存储空间的访问权限。 - private：Object是私有资源。只有Object的拥有者和授权用户有该Object的读写权限，其他用户没有权限操作该Object。 - public-read：Object是公共读资源。只有Object的拥有者和授权用户有该Object的读写权限，其他用户只有该Object的读权限。请谨慎使用该权限。 - public-read-write：Object是公共读写资源。所有用户都有该Object的读写权限。请谨慎使用该权限。 父节点：File |
| FileModifiedTime | 字符串 | 2025-05-19T16:15:33+08:00 | Object的最近一次修改时间，遵循RFC3339Nano标准格式。 父节点：File |
| ServerSideEncryption | 字符串 | AES256 | OSS创建文件时的服务器端加密编码算法。取值范围为AES256。 父节点：File |
| ServerSideEncryptionCustomerAlgorithm | 字符串 | SM4 | 用户在本地客户端加密文件时使用的加密编码算法。 父节点：File |
| ETag | 字符串 | "fba9dede5f27731c9771645a3986\\*\\*\\*\\*" | Object生成时会创建相应的ETag ，ETag用于标识一个Object的内容。 - 对于PutObject请求创建的Object，ETag值是其内容的MD5值。 - 对于其他方式创建的Object，ETag值是基于一定计算规则生成的唯一值，但不是其内容的MD5值。 **说明** ETag值可以用于检查Object内容是否发生变化。不建议使用ETag作为Object内容的MD5来校验数据完整性。 父节点：File |
| OSSCRC64 | 字符串 | 4858A48BD1466884 | Object的64位CRC值。该64位CRC根据CRC64/XZ标准计算得出。 父节点：File |
| ProduceTime | 字符串 | 2021-06-29T14:50:13.011643661+08:00 | 设备记录的照片或视频的拍摄时间。 父节点：File |
| ContentType | 字符串 | image/jpeg | MIME类型。 父节点：File |
| MediaType | 字符串 | image | 多媒体类型。 父节点：File |
| LatLong | 字符串 | 30.134390,120.074997 | 经纬度信息。 父节点：File |
| Title | 字符串 | test | 文件标题。 父节点：File |
| OSSExpiration | 字符串 | 2124-12-01T12:00:00.000Z | 文件过期时间。 父节点：File |
| AccessControlAllowOrigin | 字符串 | [https://aliyundoc.com](https://aliyundoc.com) | 允许的跨域请求的来源。 父节点：File |
| AccessControlRequestMethod | 字符串 | PUT | 跨域请求中用到的方法。 父节点：File |
| ServerSideDataEncryption | 字符串 | SM4 | Object的加密算法。 父节点：File |
| ServerSideEncryptionKeyId | 字符串 | 9468da86-3509-4f8d-a61e-6eab1eac\\*\\*\\*\\* | KMS托管的用户主密钥。 父节点：File |
| CacheControl | 字符串 | no-cache | Object被下载时网页的缓存行为。 父节点：File |
| ContentDisposition | 字符串 | attachment; filename =test.jpg | Object被下载时的名称。 父节点：File |
| ContentEncoding | 字符串 | UTF-8 | Object被下载时的内容编码格式。 父节点：File |
| ContentLanguage | 字符串 | zh-CN | Object内容使用的语言。 父节点：File |
| ImageHeight | 整型  | 500 | 图片高度，单位为像素（px）。 父节点：File |
| ImageWidth | 整型  | 270 | 图片宽度，单位为像素（px）。 父节点：File |
| VideoWidth | 整型  | 1080 | 视频画面宽度，单位为像素（px）。 父节点：File |
| VideoHeight | 整型  | 1920 | 视频画面高度，单位为像素（px）。 父节点：File |
| VideoStreams | 容器  | 不涉及 | 视频流列表。 父节点：File |
| VideoStream | 容器  | 不涉及 | 视频流。 父节点：VideoStreams |
| CodecName | 字符串 | h264 | 编码器名称。 父节点：VideoStream |
| Language | 字符串 | en  | 视频流中使用的语言，格式为BCP 47。 父节点：VideoStream |
| Bitrate | 整型  | 5407765 | 码率，单位为比特每秒（bit/s）。 父节点：VideoStream |
| FrameRate | 字符串 | 25/1 | 视频流帧率。 父节点：VideoStream |
| StartTime | 双精度浮点数 | 0.000000 | 视频流起始时间，单位为秒（s）。 父节点：VideoStream |
| Duration | 双精度浮点数 | 22.88 | 视频流持续时长，单位为秒（s）。 父节点：VideoStream |
| FrameCount | 整型  | 572 | 视频帧数。 父节点：VideoStream |
| BitDepth | 整型  | 8   | 像素位宽。 父节点：VideoStream |
| PixelFormat | 字符串 | yuv420p | 视频流像素格式。 父节点：VideoStream |
| ColorSpace | 字符串 | bt709 | 色彩空间。 父节点：VideoStream |
| Height | 整型  | 720 | 视频流画面高度，单位为像素（px）。 父节点：VideoStream |
| Width | 整型  | 1280 | 视频流画面宽度，单位为像素（px）。 父节点：VideoStream |
| AudioStreams | 容器  | 不涉及 | 音频流列表。 父节点：File |
| AudioStream | 容器  | 不涉及 | 音频流。 父节点：AudioStreams |
| CodecName | 字符串 | aac | 编码器名称。 父节点：AudioStream |
| Bitrate | 整型  | 320087 | 码率，单位为比特每秒（bit/s）。 父节点：AudioStream |
| SampleRate | 整型  | 48000 | 采样率，单位为赫兹（Hz）。 父节点：AudioStream |
| StartTime | 双精度浮点数 | 0.0235 | 音频流起始时间，单位为秒（s）。 父节点：AudioStream |
| Duration | 双精度浮点数 | 3.690667 | 音频流持续时长，单位为秒（s）。 父节点：AudioStream |
| Channels | 整型  | 2   | 声道数量。 父节点：AudioStream |
| Language | 字符串 | en  | 音频流中使用的语言，格式为BCP 47。 父节点：AudioStream |
| Subtitles | 容器  | 不涉及 | 字幕流列表。 父节点：File |
| Subtitle | 容器  | 不涉及 | 字幕流。 父节点：Subtitles |
| CodecName | 字符串 | mov\\_text | 编码器名称。 父节点：Subtitle |
| Language | 字符串 | en  | 字幕语言，格式为BCP 47。 父节点：Subtitle |
| StartTime | 双精度浮点数 | 0.000000 | 字幕流起始时间，单位为秒（s）。 父节点：Subtitle |
| Duration | 双精度浮点数 | 71.378 | 字幕流持续时长，单位为秒（s）。 父节点：Subtitle |
| Bitrate | 整型  | 13091201 | 码率，单位为比特每秒（bit/s）。 父节点：File |
| Artist | 字符串 | Jane | 艺术家。 父节点：File |
| AlbumArtist | 字符串 | Jenny | 演唱者。 父节点：File |
| Composer | 字符串 | Jane | 作曲家。 父节点：File |
| Performer | 字符串 | Jane | 演奏者。 父节点：File |
| Album | 字符串 | FirstAlbum | 专辑。 父节点：File |
| Duration | 双精度浮点数 | 15.263000 | 视频的总时长。单位秒。 父节点：File |
| Addresses | 容器  | 不涉及 | 地址信息。 父节点：File |
| Address | 容器  | 不涉及 | 地址信息。 父节点：Addresses |
| AddressLine | 字符串 | 中国浙江省杭州市余杭区文一西路969号 | 完整地址。 父节点：Address |
| City | 字符串 | 杭州市 | 城市 父节点：Address |
| District | 字符串 | 余杭区 | 区 父节点：Address |
| Language | 字符串 | zh-Hans | 语言，格式为BCP 47。 父节点：Address |
| Province | 字符串 | 浙江省 | 省份 父节点：Address |
| Township | 字符串 | 文一西路 | 街道 父节点：Address |
| OSSObjectType | 字符串 | Normal | Object的类型。 父节点：File |
| OSSStorageClass | 字符串 | Standard | Object的存储类型。 父节点：File |
| OSSTaggingCount | 整型  | 2   | Object的标签个数。 父节点：File |
| OSSTagging | 容器  | 不涉及 | 标签信息列表。 子节点：Tagging 父节点：File |
| Tagging | 容器  | 不涉及 | 单个标签信息的容器。 子节点：Key、Value 父节点：OSSTagging |
| Key | 字符串 | owner | 标签的Key。 父节点：Tagging |
| Value | 字符串 | John | 标签的Value。 父节点：Tagging |
| OSSUserMeta | 容器  | 不涉及 | 用户自定义元数据的信息列表。 子节点：UserMeta 父节点：File |
| UserMeta | 容器  | 不涉及 | 单个用户自定义元数据的容器。 子节点：Key、Value 父节点：OSSUserMeta |
| Key | 字符串 | owner | 用户自定义元数据的Key。 父节点：Tagging |
| Value | 字符串 | John | 用户自定义元数据的Value。 父节点：Tagging |
| Insights | 容器  | 不涉及 | 文件的描述信息。 子节点：Video、Image |
| Video | 容器  | 不涉及 | 视频文件的描述信息。 父节点：Insights 子节点：Caption、Description |
| Caption | 字符串 | 不涉及 | 简短描述信息。 父节点：Video |
| Image | 容器  | 不涉及 | 图片文件的描述信息 父节点：Insights 子节点：Caption、Description |
| Caption | 字符串 | 不涉及 | 简短描述信息。 父节点：Image |
| Description | 字符串 | 不涉及 | 详细描述信息。 父节点：Image |

## 示例

### **请求示例**

#### **标量检索**

```
POST /?metaQuery&comp=query&mode=basic HTTP/1.1
Host: BucketName.oss-cn-hangzhou.aliyuncs.com
Date: GMT Date
Authorization: SignatureValue 
<?xml version="1.0" encoding="UTF-8"?>
<MetaQuery>
  <NextToken></NextToken>
  <MaxResults>5</MaxResults>
  <Query>{"Field": "Size","Value": "1048576","Operation": "gt"}</Query>
  <Sort>Size</Sort>
  <Order>asc</Order>
  <Aggregations>
    <Aggregation>
      <Field>Size</Field>
      <Operation>sum</Operation>
    </Aggregation>
    <Aggregation>
      <Field>Size</Field>
      <Operation>max</Operation>
    </Aggregation>
  </Aggregations>
</MetaQuery>
```

#### **向量检索**

```
POST /?metaQuery&comp=query&mode=semantic HTTP/1.1
Host: BucketName.oss-cn-hangzhou.aliyuncs.com
Date: Thu, 12 Sep 2024 13:08:38 GMT
Authorization: SignatureValue 
<?xml version="1.0" encoding="UTF-8"?>
<MetaQuery>
  <MaxResults>99</MaxResults>
  <Query>俯瞰白雪覆盖的森林</Query> // 必选
  <MediaTypes>
    <MediaType>image</MediaType>
  </MediaTypes>
  // SimpleQuery等同于Simple模式下的Query字段。
  <SimpleQuery>{"Operation":"gt", "Field": "Size", "Value": "30"}</SimpleQuery>
</MetaQuery>
```

### **返回示例**

#### **标量检索**

```
HTTP/1.1 200 OK
x-oss-request-id: 5C1B138A109F4E405B2D****
Date: Mon, 26 Jul 2021 13:08:38 GMT
Content-Length: 118
Content-Type: application/xml
Connection: keep-alive
Server: AliyunOSS
<?xml version="1.0" encoding="UTF-8"?>
<MetaQuery>
  <NextToken>MTIzNDU2Nzg6aW1tdGVzdDpleGFtcGxlYnVja2V0OmRhdGFzZXQwMDE6b3NzOi8vZXhhbXBsZWJ1Y2tldC9zYW1wbGVvYmplY3QxLmpw****</NextToken>
  <Files>
    <File>
      <Filename>exampleobject.txt</Filename>
      <Size>120</Size>
      <FileModifiedTime>2025-05-19T16:14:38+08:00</FileModifiedTime>
      <OSSObjectType>Normal</OSSObjectType>
      <OSSStorageClass>Standard</OSSStorageClass>
      <ObjectACL>default</ObjectACL>
      <ETag>"fba9dede5f27731c9771645a3986****"</ETag>
      <OSSCRC64>4858A48BD1466884</OSSCRC64>
      <OSSTaggingCount>2</OSSTaggingCount>
      <OSSTagging>
        <Tagging>
          <Key>owner</Key>
          <Value>John</Value>
        </Tagging>
        <Tagging>
          <Key>type</Key>
          <Value>document</Value>
        </Tagging>
      </OSSTagging>
      <OSSUserMeta>
        <UserMeta>
          <Key>x-oss-meta-location</Key>
          <Value>hangzhou</Value>
        </UserMeta>
      </OSSUserMeta>
    </File>
  </Files>
</MetaQuery>
```

#### **向量检索**

##### **检索图片返回示例**

```
HTTP/1.1 200 OK
x-oss-request-id: 5C1B138A109F4E405B2D****
Date: Thu, 12 Sep 2024 13:08:38 GMT
Content-Length: 118
Content-Type: application/xml
Connection: keep-alive
Server: AliyunOSS
<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<MetaQuery>
  <Files>
    <File>
      <URI>oss://examplebucket/test-object.jpg</URI>
      <Filename>sampleobject.jpg</Filename>
      <Size>1000</Size>
      <ObjectACL>default</ObjectACL>
      <FileModifiedTime>2025-05-19T16:14:38+08:00</FileModifiedTime>
      <ServerSideEncryption>AES256</ServerSideEncryption>
      <ServerSideEncryptionCustomerAlgorithm>SM4</ServerSideEncryptionCustomerAlgorithm>
      <ETag>\"1D9C280A7C4F67F7EF873E28449****\"</ETag>
      <OSSCRC64>559890638950338001</OSSCRC64>
      <ProduceTime>2021-06-29T14:50:15.011643661+08:00</ProduceTime>
      <ContentType>image/jpeg</ContentType>
      <MediaType>image</MediaType>
      <LatLong>30.134390,120.074997</LatLong>
      <Title>test</Title>
      <OSSExpiration>2024-12-01T12:00:00.000Z</OSSExpiration>
      <AccessControlAllowOrigin>https://aliyundoc.com</AccessControlAllowOrigin>
      <AccessControlRequestMethod>PUT</AccessControlRequestMethod>
      <ServerSideDataEncryption>SM4</ServerSideDataEncryption>
      <ServerSideEncryptionKeyId>9468da86-3509-4f8d-a61e-6eab1eac****</ServerSideEncryptionKeyId>
      <CacheControl>no-cache</CacheControl>
      <ContentDisposition>attachment; filename =test.jpg</ContentDisposition>
      <ContentEncoding>UTF-8</ContentEncoding>
      <ContentLanguage>zh-CN</ContentLanguage>
      <ImageHeight>500</ImageHeight>
      <ImageWidth>270</ImageWidth>
      <Addresses>
        <Address>
          <AddressLine>中国浙江省杭州市余杭区文一西路969号</AddressLine>
          <City>杭州市</City>
          <Country>中国</Country>
          <District>余杭区</District>
          <Language>zh-Hans</Language>
          <Province>浙江省</Province>
          <Township>文一西路</Township>
        </Address>
        <Address>
          <AddressLine>中国浙江省杭州市余杭区文一西路970号</AddressLine>
          <City>杭州市</City>
          <Country>中国</Country>
          <District>余杭区</District>
          <Language>zh-Hans</Language>
          <Province>浙江省</Province>
          <Township>文一西路</Township>
        </Address>
      </Addresses>
      <OSSObjectType>Normal</OSSObjectType>
      <OSSStorageClass>Standard</OSSStorageClass>
      <OSSTaggingCount>2</OSSTaggingCount>
      <OSSTagging>
        <Tagging>
          <Key>key</Key>
          <Value>val</Value>
        </Tagging>
        <Tagging>
          <Key>key</Key>
          <Value>val</Value>
        </Tagging>
      </OSSTagging>
      <OSSUserMeta>
        <UserMeta>
          <Key>key</Key>
          <Value>val</Value>
        </UserMeta>
      </OSSUserMeta>
      <Insights>
        <Image>
          <Caption>站着一个人</Caption>
          <Description>图片中有一人，穿着深色西装外套，内搭白色衬衫。背景为渐变的浅蓝色至灰色</Description>
        </Image>
      </Insights>
    </File>
  </Files>
</MetaQuery>
```

##### **检索音视频返回示例**

```
HTTP/1.1 200 OK
x-oss-request-id: 5C1B138A109F4E405B2D****
Date: Thu, 12 Sep 2024 13:08:38 GMT
Content-Length: 118
Content-Type: application/xml
Connection: keep-alive
Server: AliyunOSS
<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<MetaQuery>
  <Files>
    <File>
      <URI>oss://examplebucket/test-object.jpg</URI>
      <Filename>sampleobject.jpg</Filename>
      <Size>1000</Size>
      <ObjectACL>default</ObjectACL>
      <FileModifiedTime>2021-06-29T14:50:14.011643661+08:00</FileModifiedTime>
      <ServerSideEncryption>AES256</ServerSideEncryption>
      <ServerSideEncryptionCustomerAlgorithm>SM4</ServerSideEncryptionCustomerAlgorithm>
      <ETag>\"1D9C280A7C4F67F7EF873E28449****\"</ETag>
      <OSSCRC64>559890638950338001</OSSCRC64>
      <ProduceTime>2021-06-29T14:50:15.011643661+08:00</ProduceTime>
      <ContentType>image/jpeg</ContentType>
      <MediaType>image</MediaType>
      <LatLong>30.134390,120.074997</LatLong>
      <Title>test</Title>
      <OSSExpiration>2024-12-01T12:00:00.000Z</OSSExpiration>
      <AccessControlAllowOrigin>https://aliyundoc.com</AccessControlAllowOrigin>
      <AccessControlRequestMethod>PUT</AccessControlRequestMethod>
      <ServerSideDataEncryption>SM4</ServerSideDataEncryption>
      <ServerSideEncryptionKeyId>9468da86-3509-4f8d-a61e-6eab1eac****</ServerSideEncryptionKeyId>
      <CacheControl>no-cache</CacheControl>
      <ContentDisposition>attachment; filename =test.jpg</ContentDisposition>
      <ContentEncoding>UTF-8</ContentEncoding>
      <ContentLanguage>zh-CN</ContentLanguage>
      <VideoWidth>1080</VideoWidth>
      <VideoHeight>1920</VideoHeight>
      <VideoStreams>
        <VideoStream>
          <CodecName>h264</CodecName>
          <Language>en</Language>
          <Bitrate>5407765</Bitrate>
          <FrameRate>25/1</FrameRate>
          <StartTime>0</StartTime>
          <Duration>22.88</Duration>
          <FrameCount>572</FrameCount>
          <BitDepth>8</BitDepth>
          <PixelFormat>yuv420p</PixelFormat>
          <ColorSpace>bt709</ColorSpace>
          <Height>720</Height>
          <Width>1280</Width>
        </VideoStream>
        <VideoStream>
          <CodecName>h264</CodecName>
          <Language>en</Language>
          <Bitrate>5407765</Bitrate>
          <FrameRate>25/1</FrameRate>
          <StartTime>0</StartTime>
          <Duration>22.88</Duration>
          <FrameCount>572</FrameCount>
          <BitDepth>8</BitDepth>
          <PixelFormat>yuv420p</PixelFormat>
          <ColorSpace>bt709</ColorSpace>
          <Height>720</Height>
          <Width>1280</Width>
        </VideoStream>
      </VideoStreams>
      <AudioStreams>
        <AudioStream>
          <CodecName>aac</CodecName>
          <Bitrate>1048576</Bitrate>
          <SampleRate>48000</SampleRate>
          <StartTime>0.0235</StartTime>
          <Duration>3.690667</Duration>
          <Channels>2</Channels>
          <Language>en</Language>
        </AudioStream>
      </AudioStreams>
      <Subtitles>
        <Subtitle>
          <CodecName>mov_text</CodecName>
          <Language>en</Language>
          <StartTime>0</StartTime>
          <Duration>71.378</Duration>
        </Subtitle>
        <Subtitle>
          <CodecName>mov_text</CodecName>
          <Language>en</Language>
          <StartTime>72</StartTime>
          <Duration>71.378</Duration>
        </Subtitle>
      </Subtitles>
      <Bitrate>5407765</Bitrate>
      <Artist>Jane</Artist>
      <AlbumArtist>Jenny</AlbumArtist>
      <Composer>Jane</Composer>
      <Performer>Jane</Performer>
      <Album>FirstAlbum</Album>
      <Duration>71.378</Duration>
      <Addresses>
        <Address>
          <AddressLine>中国浙江省杭州市余杭区文一西路969号</AddressLine>
          <City>杭州市</City>
          <Country>中国</Country>
          <District>余杭区</District>
          <Language>zh-Hans</Language>
          <Province>浙江省</Province>
          <Township>文一西路</Township>
        </Address>
        <Address>
          <AddressLine>中国浙江省杭州市余杭区文一西路970号</AddressLine>
          <City>杭州市</City>
          <Country>中国</Country>
          <District>余杭区</District>
          <Language>zh-Hans</Language>
          <Province>浙江省</Province>
          <Township>文一西路</Township>
        </Address>
      </Addresses>
      <OSSObjectType>Normal</OSSObjectType>
      <OSSStorageClass>Standard</OSSStorageClass>
      <OSSTaggingCount>2</OSSTaggingCount>
      <OSSTagging>
        <Tagging>
          <Key>key</Key>
          <Value>val</Value>
        </Tagging>
        <Tagging>
          <Key>key</Key>
          <Value>val</Value>
        </Tagging>
      </OSSTagging>
      <OSSUserMeta>
        <UserMeta>
          <Key>key</Key>
          <Value>val</Value>
        </UserMeta>
      </OSSUserMeta>
      <Insights>
        <Video>
          <Caption>视频中展示了两个不同场景：一个是静止的白色盘子、黑色瓶子和透明玻璃杯，另一个是手拿着标有“YEZOLU”的洗发水瓶在浴室中缓慢上移</Caption>
        </Video>
      </Insights>
    </File>
  </Files>
</MetaQuery>
```

## Query示例

Query支持嵌套使用，您可以通过Query的嵌套构建复杂的查询条件，精确查询到所需内容。

-   如果要搜索名称为exampleobject.txt且大小小于1000字节的文件，则Query填写示例如下：
    
    ```
    
    {
      "SubQueries":[
        {
          "Field":"Filename",
          "Value": "exampleobject.txt",
          "Operation":"eq"
        },         
        {
          "Field":"Size",
          "Value":"1000",
          "Operation":"lt"
        }
      ],
      "Operation":"and"
    }
                
    ```
    
-   如果要搜索以`exampledir/`为前缀，包含`type=document`或`owner=John`标签且大小大于10 MB的文件，则Query填写示例如下：
    
    ```
    
    {
      "SubQueries": [
        {
          "Field": "Filename",
          "Value": "exampledir/",
          "Operation": "prefix"
        },
        {
          "Field": "Size",
          "Value": "1048576",
          "Operation": "gt"
        },
        {
          "SubQueries": [
            {
              "Field": "OSSTagging.type",
              "Value": "document",
              "Operation": "eq"
            },
            {
              "Field": "OSSTagging.owner",
              "Value": "John",
              "Operation": "eq"
            }
          ],
          "Operation": "or"
        }
      ],
      "Operation": "and"
    }
            
                
    ```
    

结合以上搜索条件，您还可以通过聚合操作实现不同数据的统计和分析，例如计算符合搜索条件的所有文件的大小总和、数量、平均值或者最值，统计所有符合搜索条件图片的尺寸分布情况。

## SDK

本接口对应的各语言SDK如下：

-   [Java](https://help.aliyun.com/zh/oss/developer-reference/data-indexing-2)
    
-   [Python V2](https://help.aliyun.com/zh/oss/developer-reference/data-indexing-using-oss-sdk-for-python-v2)
    
-   [Go V2](https://help.aliyun.com/zh/oss/developer-reference/data-indexing-3)
    
-   [PHP V2](https://help.aliyun.com/zh/oss/developer-reference/data-indexing-using-oss-sdk-for-php-v2)
    

## **命令行工具ossutil**

DoMetaQuery接口所对应的ossutil命令为[do-meta-query](https://help.aliyun.com/zh/oss/developer-reference/do-meta-query)。
