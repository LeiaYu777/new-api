/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import { Button, Form } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';

import {
  TASK_ACTION_FIRST_TAIL_GENERATE,
  TASK_ACTION_GENERATE,
  TASK_ACTION_REFERENCE_GENERATE,
  TASK_ACTION_REMIX_GENERATE,
  TASK_ACTION_TEXT_GENERATE,
} from '../../../constants/common.constant';
import { DATE_RANGE_PRESETS } from '../../../constants/console.constants';

const STATUS_OPTIONS = [
  { value: 'NOT_START', label: '未启动' },
  { value: 'SUBMITTED', label: '队列中' },
  { value: 'QUEUED', label: '排队中' },
  { value: 'IN_PROGRESS', label: '执行中' },
  { value: 'SUCCESS', label: '成功' },
  { value: 'FAILURE', label: '失败' },
];

const ACTION_OPTIONS = [
  { value: TASK_ACTION_TEXT_GENERATE, label: '文生视频' },
  { value: TASK_ACTION_GENERATE, label: '图生视频' },
  { value: TASK_ACTION_FIRST_TAIL_GENERATE, label: '首尾生视频' },
  { value: TASK_ACTION_REFERENCE_GENERATE, label: '参照生视频' },
  { value: TASK_ACTION_REMIX_GENERATE, label: '视频Remix' },
  { value: 'MUSIC', label: '生成音乐' },
  { value: 'LYRICS', label: '生成歌词' },
];

const PLATFORM_OPTIONS = [
  { value: '54', label: 'DoubaoVideo / Seedance' },
  { value: '45', label: 'VolcEngine / 火山方舟' },
  { value: 'suno', label: 'Suno' },
  { value: 'mj', label: 'Midjourney' },
];

const BILLING_SOURCE_OPTIONS = [
  { value: 'wallet', label: '钱包' },
  { value: 'subscription', label: '订阅' },
];

const TaskLogsFilters = ({
  formInitValues,
  setFormApi,
  refresh,
  setShowColumnSelector,
  formApi,
  loading,
  isAdminUser,
  t,
}) => {
  return (
    <Form
      initValues={formInitValues}
      getFormApi={(api) => setFormApi(api)}
      onSubmit={refresh}
      allowEmpty={true}
      autoComplete='off'
      layout='vertical'
      trigger='change'
      stopValidateWithError={false}
    >
      <div className='flex flex-col gap-2'>
        <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-2'>
          {/* 时间选择器 */}
          <div className='col-span-1 lg:col-span-2'>
            <Form.DatePicker
              field='dateRange'
              className='w-full'
              type='dateTimeRange'
              placeholder={[t('开始时间'), t('结束时间')]}
              showClear
              pure
              size='small'
              presets={DATE_RANGE_PRESETS.map((preset) => ({
                text: t(preset.text),
                start: preset.start(),
                end: preset.end(),
              }))}
            />
          </div>

          {/* 任务 ID */}
          <Form.Input
            field='task_id'
            prefix={<IconSearch />}
            placeholder={t('任务 ID')}
            showClear
            pure
            size='small'
          />

          {/* 渠道 ID - 仅管理员可见 */}
          {isAdminUser && (
            <Form.Input
              field='channel_id'
              prefix={<IconSearch />}
              placeholder={t('渠道 ID')}
              showClear
              pure
              size='small'
            />
          )}

          <Form.Select
            field='platform'
            placeholder={t('平台')}
            showClear
            pure
            size='small'
          >
            {PLATFORM_OPTIONS.map((option) => (
              <Form.Select.Option key={option.value} value={option.value}>
                {t(option.label)}
              </Form.Select.Option>
            ))}
          </Form.Select>

          <Form.Select
            field='action'
            placeholder={t('类型')}
            showClear
            pure
            size='small'
          >
            {ACTION_OPTIONS.map((option) => (
              <Form.Select.Option key={option.value} value={option.value}>
                {t(option.label)}
              </Form.Select.Option>
            ))}
          </Form.Select>

          <Form.Select
            field='status'
            placeholder={t('任务状态')}
            showClear
            pure
            size='small'
          >
            {STATUS_OPTIONS.map((option) => (
              <Form.Select.Option key={option.value} value={option.value}>
                {t(option.label)}
              </Form.Select.Option>
            ))}
          </Form.Select>

          {isAdminUser && (
            <Form.Select
              field='billing_source'
              placeholder={t('资金来源')}
              showClear
              pure
              size='small'
            >
              {BILLING_SOURCE_OPTIONS.map((option) => (
                <Form.Select.Option key={option.value} value={option.value}>
                  {t(option.label)}
                </Form.Select.Option>
              ))}
            </Form.Select>
          )}
        </div>

        {/* 操作按钮区域 */}
        <div className='flex justify-between items-center'>
          <div></div>
          <div className='flex gap-2'>
            <Button
              type='tertiary'
              htmlType='submit'
              loading={loading}
              size='small'
            >
              {t('查询')}
            </Button>
            <Button
              type='tertiary'
              onClick={() => {
                if (formApi) {
                  formApi.reset();
                  // 重置后立即查询，使用setTimeout确保表单重置完成
                  setTimeout(() => {
                    refresh();
                  }, 100);
                }
              }}
              size='small'
            >
              {t('重置')}
            </Button>
            <Button
              type='tertiary'
              onClick={() => setShowColumnSelector(true)}
              size='small'
            >
              {t('列设置')}
            </Button>
          </div>
        </div>
      </div>
    </Form>
  );
};

export default TaskLogsFilters;
