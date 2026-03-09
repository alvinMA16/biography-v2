-- 重命名 profile_completed 为 onboarding_completed
-- 更准确地反映该字段的含义：首次对话是否完成

ALTER TABLE users RENAME COLUMN profile_completed TO onboarding_completed;
