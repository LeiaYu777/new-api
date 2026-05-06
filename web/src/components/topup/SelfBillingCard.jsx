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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Card, Empty, Form, Tag, Typography } from '@douyinfe/semi-ui';
import { IconDownload, IconRefresh, IconSearch } from '@douyinfe/semi-icons';
import { ReceiptText } from 'lucide-react';
import CardTable from '../common/ui/CardTable';
import {
  API,
  getTodayStartTimestamp,
  renderQuota,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';

const { Text, Title } = Typography;

const DEFAULT_MODEL_FILTER = 'doubao-seedance-2-0%';
const DEFAULT_LIMIT = 100;

const BILLING_SOURCE_OPTIONS = [
  { value: '', label: '全部资金来源' },
  { value: 'wallet', label: '钱包余额' },
  { value: 'subscription', label: '订阅额度' },
];

const LOG_TYPE_OPTIONS = [
  { value: 0, label: '消费与退款' },
  { value: 1, label: '充值' },
  { value: 2, label: '消费' },
  { value: 6, label: '退款' },
];

const BILLING_SOURCE_LABELS = {
  wallet: '钱包余额',
  subscription: '订阅额度',
};

const initialDateRange = () => {
  const now = Math.floor(Date.now() / 1000);
  return [
    timestamp2string(getTodayStartTimestamp()),
    timestamp2string(now + 3600),
  ];
};

const trimValue = (value) => {
  if (value === undefined || value === null) return '';
  return String(value).trim();
};

const toUnixSeconds = (value) => {
  if (!value) return 0;
  if (value instanceof Date) return Math.floor(value.getTime() / 1000);
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) return 0;
  return Math.floor(parsed / 1000);
};

const buildParams = (values = {}) => {
  const dateRange = Array.isArray(values.dateRange)
    ? values.dateRange
    : initialDateRange();
  const params = {
    start_timestamp: toUnixSeconds(dateRange[0]),
    end_timestamp: toUnixSeconds(dateRange[1]),
    model_name: trimValue(values.model_name),
    channel: Number(values.channel || 0),
    group: trimValue(values.group),
    task_id: trimValue(values.task_id),
    billing_source: trimValue(values.billing_source),
    limit: Number(values.limit || DEFAULT_LIMIT),
  };

  return Object.fromEntries(
    Object.entries(params).filter(([, value]) => {
      if (typeof value === 'number') return value > 0;
      return value !== '';
    }),
  );
};

const formatCount = (value) =>
  Number(value || 0).toLocaleString(undefined, { maximumFractionDigits: 0 });

const SelfBillingCard = ({ t }) => {
  const [formApi, setFormApi] = useState(null);
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [summary, setSummary] = useState({
    items: [],
    consume_quota: 0,
    refund_quota: 0,
    net_quota: 0,
    request_count: 0,
    refund_count: 0,
    total_tokens: 0,
  });
  const [statements, setStatements] = useState({ items: [], total: 0 });
  const [statementsLoading, setStatementsLoading] = useState(false);

  const formInitValues = useMemo(
    () => ({
      dateRange: initialDateRange(),
      model_name: DEFAULT_MODEL_FILTER,
      billing_source: '',
      logType: 0,
      limit: DEFAULT_LIMIT,
    }),
    [],
  );

  const getCurrentValues = useCallback(() => {
    if (!formApi) return formInitValues;
    return { ...formInitValues, ...formApi.getValues() };
  }, [formApi, formInitValues]);

  const loadSummary = useCallback(
    async (values) => {
      setLoading(true);
      try {
        const res = await API.get('/api/billing/self/summary', {
          params: buildParams(values || getCurrentValues()),
        });
        if (res.data.success) {
          const data = res.data.data || {};
          setSummary({
            items: Array.isArray(data.items) ? data.items : [],
            consume_quota: data.consume_quota || 0,
            refund_quota: data.refund_quota || 0,
            net_quota: data.net_quota || 0,
            request_count: data.request_count || 0,
            refund_count: data.refund_count || 0,
            total_tokens: data.total_tokens || 0,
          });
        } else {
          showError(res.data.message || t('账单汇总查询失败'));
        }
      } catch (error) {
        showError(error);
      } finally {
        setLoading(false);
      }
    },
    [getCurrentValues, t],
  );

  const loadStatements = useCallback(
    async (values) => {
      setStatementsLoading(true);
      try {
        const res = await API.get('/api/billing/self/statements', {
          params: {
            ...buildParams(values || getCurrentValues()),
            p: 1,
            page_size: 20,
          },
        });
        if (res.data.success) {
          const data = res.data.data || {};
          setStatements({
            items: Array.isArray(data.items) ? data.items : [],
            total: data.total || 0,
          });
        } else {
          showError(res.data.message || t('月结快照查询失败'));
        }
      } catch (error) {
        showError(error);
      } finally {
        setStatementsLoading(false);
      }
    },
    [getCurrentValues, t],
  );

  const handleSearch = useCallback(
    async (values) => {
      await Promise.all([loadSummary(values), loadStatements(values)]);
    },
    [loadSummary, loadStatements],
  );

  useEffect(() => {
    if (formApi) {
      handleSearch(formInitValues);
    }
  }, [formApi, formInitValues, handleSearch]);

  const resetFilters = () => {
    formApi?.reset();
    setTimeout(() => handleSearch(formInitValues), 0);
  };

  const downloadBillingCsv = async () => {
    setExporting(true);
    try {
      const values = getCurrentValues();
      const params = buildParams(values);
      const logType = Number(values.logType || 0);
      if (logType > 0) {
        params.type = logType;
      }
      if (logType === 1) {
        delete params.model_name;
        delete params.channel;
        delete params.group;
        delete params.billing_source;
        delete params.task_id;
      }
      const res = await API.get('/api/billing/self/export', {
        params,
        responseType: 'blob',
        disableDuplicate: true,
      });
      const blob = new Blob([res.data], {
        type: 'text/csv;charset=utf-8',
      });
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `my-seedance-billing-${Date.now()}.csv`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
      showSuccess(t('账单流水已导出'));
    } catch (error) {
      showError(error);
    } finally {
      setExporting(false);
    }
  };

  const stats = [
    {
      title: t('本期消费'),
      value: renderQuota(summary.consume_quota, 6),
      desc: t('Seedance 消费扣费合计'),
    },
    {
      title: t('本期退款'),
      value: renderQuota(summary.refund_quota, 6),
      desc: t('失败、取消或回滚返还'),
    },
    {
      title: t('净消耗'),
      value: renderQuota(summary.net_quota, 6),
      desc: t('消费扣费减去退款'),
    },
    {
      title: t('调用次数'),
      value: formatCount(summary.request_count),
      desc: t('消费流水条数'),
    },
  ];

  const columns = [
    {
      title: t('模型'),
      dataIndex: 'model_name',
      key: 'model_name',
      width: 220,
      render: (value) => <Text copyable>{value || '-'}</Text>,
    },
    {
      title: t('渠道'),
      dataIndex: 'channel_id',
      key: 'channel_id',
      width: 90,
      render: (value) => value || '-',
    },
    {
      title: t('分组'),
      dataIndex: 'group',
      key: 'group',
      width: 120,
      render: (value) => (value ? <Tag>{value}</Tag> : '-'),
    },
    {
      title: t('资金来源'),
      dataIndex: 'billing_source',
      key: 'billing_source',
      width: 130,
      render: (value) => {
        const source = value || 'wallet';
        return (
          <Tag color={source === 'subscription' ? 'green' : 'blue'}>
            {t(BILLING_SOURCE_LABELS[source] || source)}
          </Tag>
        );
      },
    },
    {
      title: t('消费扣费'),
      dataIndex: 'consume_quota',
      key: 'consume_quota',
      width: 140,
      render: (value) => renderQuota(value, 6),
    },
    {
      title: t('退款返还'),
      dataIndex: 'refund_quota',
      key: 'refund_quota',
      width: 140,
      render: (value) => renderQuota(value, 6),
    },
    {
      title: t('净扣费'),
      dataIndex: 'net_quota',
      key: 'net_quota',
      width: 140,
      render: (value) => <Text strong>{renderQuota(value, 6)}</Text>,
    },
    {
      title: t('调用次数'),
      dataIndex: 'request_count',
      key: 'request_count',
      width: 110,
      render: (value) => formatCount(value),
    },
  ];

  const statementColumns = [
    {
      title: t('账期开始'),
      dataIndex: 'period_start',
      key: 'period_start',
      width: 170,
      render: (value) => timestamp2string(value),
    },
    {
      title: t('账期结束'),
      dataIndex: 'period_end',
      key: 'period_end',
      width: 170,
      render: (value) => timestamp2string(value),
    },
    {
      title: t('模型'),
      dataIndex: 'model_name',
      key: 'model_name',
      width: 220,
      render: (value) => <Text copyable>{value || '-'}</Text>,
    },
    {
      title: t('资金来源'),
      dataIndex: 'billing_source',
      key: 'billing_source',
      width: 130,
      render: (value) => {
        const source = value || 'wallet';
        return (
          <Tag color={source === 'subscription' ? 'green' : 'blue'}>
            {t(BILLING_SOURCE_LABELS[source] || source)}
          </Tag>
        );
      },
    },
    {
      title: t('净扣费'),
      dataIndex: 'net_quota',
      key: 'net_quota',
      width: 140,
      render: (value) => <Text strong>{renderQuota(value, 6)}</Text>,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (value) => <Tag>{value || 'closed'}</Tag>,
    },
  ];

  return (
    <Card className='!rounded-2xl shadow-sm border-0 mt-6'>
      <div className='flex flex-col lg:flex-row lg:items-start justify-between gap-3 mb-4'>
        <div className='flex items-start gap-3'>
          <div className='mt-1 flex h-9 w-9 items-center justify-center rounded-xl bg-[var(--semi-color-fill-0)]'>
            <ReceiptText size={18} />
          </div>
          <div>
            <Title heading={5} className='!mb-1'>
              {t('我的 Seedance 账单')}
            </Title>
            <Text type='secondary'>
              {t(
                '查询自己的 Seedance 2.0 消费、退款、钱包余额和订阅额度扣减情况；数据范围会自动绑定当前账号。',
              )}
            </Text>
          </div>
        </div>
        <div className='flex gap-2 justify-end'>
          <Button
            type='tertiary'
            icon={<IconRefresh />}
            loading={loading || statementsLoading}
            onClick={() => handleSearch(getCurrentValues())}
          >
            {t('刷新')}
          </Button>
          <Button
            type='primary'
            icon={<IconDownload />}
            loading={exporting}
            onClick={downloadBillingCsv}
          >
            {t('导出我的流水')}
          </Button>
        </div>
      </div>

      <Form
        initValues={formInitValues}
        getFormApi={setFormApi}
        onSubmit={handleSearch}
        allowEmpty
        autoComplete='off'
        layout='vertical'
        trigger='change'
      >
        <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-2 mb-4'>
          <div className='col-span-1 xl:col-span-2'>
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
          <Form.Input
            field='model_name'
            prefix={<IconSearch />}
            placeholder={t('模型名称，支持 % 通配')}
            showClear
            pure
            size='small'
          />
          <Form.Input
            field='task_id'
            prefix={<IconSearch />}
            placeholder={t('任务 ID 精确对账')}
            showClear
            pure
            size='small'
          />
          <Form.Select
            field='billing_source'
            placeholder={t('资金来源')}
            pure
            size='small'
          >
            {BILLING_SOURCE_OPTIONS.map((option) => (
              <Form.Select.Option key={option.value} value={option.value}>
                {t(option.label)}
              </Form.Select.Option>
            ))}
          </Form.Select>
          <Form.Select
            field='logType'
            placeholder={t('导出类型')}
            pure
            size='small'
          >
            {LOG_TYPE_OPTIONS.map((option) => (
              <Form.Select.Option key={option.value} value={option.value}>
                {t(option.label)}
              </Form.Select.Option>
            ))}
          </Form.Select>
          <Form.InputNumber
            field='channel'
            prefix={<IconSearch />}
            placeholder={t('渠道 ID')}
            min={1}
            pure
            size='small'
          />
          <Form.Input
            field='group'
            prefix={<IconSearch />}
            placeholder={t('分组')}
            showClear
            pure
            size='small'
          />
          <Form.InputNumber
            field='limit'
            placeholder={t('返回条数')}
            min={1}
            max={10000}
            pure
            size='small'
          />
        </div>
        <div className='flex justify-end gap-2 mb-4'>
          <Button
            type='tertiary'
            htmlType='submit'
            icon={<IconSearch />}
            loading={loading || statementsLoading}
            size='small'
          >
            {t('查询')}
          </Button>
          <Button
            type='tertiary'
            icon={<IconRefresh />}
            onClick={resetFilters}
            size='small'
          >
            {t('重置')}
          </Button>
        </div>
      </Form>

      <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-2 mb-4'>
        {stats.map((item) => (
          <Card key={item.title} className='!rounded-xl' bordered>
            <div className='flex flex-col gap-1'>
              <Text type='secondary' size='small'>
                {item.title}
              </Text>
              <Text strong className='text-lg'>
                {item.value}
              </Text>
              <Text type='tertiary' size='small'>
                {item.desc}
              </Text>
            </div>
          </Card>
        ))}
      </div>

      <CardTable
        columns={columns}
        dataSource={(summary.items || []).map((item, index) => ({
          ...item,
          key: `${item.model_name}-${item.channel_id}-${item.group}-${item.billing_source}-${index}`,
        }))}
        rowKey='key'
        loading={loading}
        scroll={{ x: 'max-content' }}
        size='small'
        className='rounded-xl overflow-hidden'
        empty={<Empty description={t('暂无账单汇总')} />}
        pagination={false}
      />

      <div className='mt-5 mb-3 flex flex-col md:flex-row md:items-center justify-between gap-2'>
        <div>
          <Title heading={6} className='!mb-1'>
            {t('我的月结快照')}
          </Title>
          <Text type='secondary'>
            {t('如果管理员已生成月结快照，这里会显示当前账号对应账期记录。')}
          </Text>
        </div>
        <Text type='tertiary' size='small'>
          {t('共 {{total}} 条快照', { total: statements.total || 0 })}
        </Text>
      </div>
      <CardTable
        columns={statementColumns}
        dataSource={(statements.items || []).map((item) => ({
          ...item,
          key: item.id,
        }))}
        rowKey='key'
        loading={statementsLoading}
        scroll={{ x: 'max-content' }}
        size='small'
        className='rounded-xl overflow-hidden'
        empty={<Empty description={t('暂无月结快照')} />}
        pagination={false}
      />
    </Card>
  );
};

export default SelfBillingCard;
