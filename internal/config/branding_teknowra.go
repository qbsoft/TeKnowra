package config

import "bytes"

// 提示词模板的品牌替换 —— TeKnowra fork 自己的文件，上游没有。
//
// 上游的 config/prompt_templates/*.yaml 里，每个人设都以
// "You are WeKnora, ... developed by Tencent" 开头。模型会照着这句话向用户
// 自我介绍（问「你是谁」就答「我是腾讯开发的 WeKnora」），所以它属于用户可见的品牌。
//
// 为什么在加载时换，而不是直接改 yaml：
//   - 那些 yaml 是上游改得很勤的文件，逐行改意味着每次合并上游都要解冲突；
//     这里只在 loadPromptTemplates 里挂一行调用。
//   - 上游以后新增人设会自动被换掉，不用追着补。
//
// 只替换大小写完全一致的 "WeKnora"。小写的 ".weknora/requirements.json" 之类是
// 技能机制真实依赖的路径，不是品牌，绝不能动 —— branding_teknowra_test.go 锁着这一条。
// 完整的品牌改造清单见仓库根目录 BRANDING.md。
var promptBrandReplacements = [][2][]byte{
	{[]byte("developed by Tencent"), []byte("developed by TyerSoft")},
	{[]byte("WeKnora"), []byte("TeKnowra")},
}

// rebrandPromptTemplate 对一份提示词模板的原始内容做品牌替换。
func rebrandPromptTemplate(data []byte) []byte {
	for _, r := range promptBrandReplacements {
		data = bytes.ReplaceAll(data, r[0], r[1])
	}
	return data
}
