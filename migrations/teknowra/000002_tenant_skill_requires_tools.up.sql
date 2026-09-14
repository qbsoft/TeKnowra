-- 租户技能带上 requires_tools。
--
-- 上游把技能装进沙箱镜像、元数据存 tenant_skills 表，只存了名字、简介、正文
-- 三样。SKILL.md 开头声明的 requires_tools 在安装时**已经被解析出来**
-- （ParseSkillBundle → ParseSkillFile），只是没有列可落，于是丢在半路。
--
-- 丢了它，「技能声明了要用哪些工具，没给就提醒」这个保护在租户技能这条路上
-- 就不存在了。而它防的是一种不报错的失败：模型读完说明书、发现工具不在、
-- 自己编一个答案出来，看着还挺像回事。
--
-- 用 JSONB 而不是关联表：这是一份跟着技能走的声明，永远整读整写，没有
-- 按工具名反查的需求。真要反查，jsonb 也能建 GIN 索引。
ALTER TABLE tenant_skills
    ADD COLUMN IF NOT EXISTS requires_tools JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN tenant_skills.requires_tools IS
    'SKILL.md frontmatter 里 requires_tools 的原样副本：这个技能正文点名调用的工具，用工具自己的名字。空数组表示作者没声明，不等于不需要工具。';
