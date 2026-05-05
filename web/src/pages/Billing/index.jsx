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
import { useTranslation } from 'react-i18next';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
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

const LOG_TYPE_OPTIONS = [
  { value: 0, label: '全部流水' },
  { value: 1, label: '充值' },
  { value: 2, label: '消费' },
  { value: 6, label: '退款' },
];

const BILLING_SOURCE_OPTIONS = [
  { value: '', label: '全部资金来源' },
  { value: 'wallet', label: '钱包余额' },
  { value: 'subscription', label: '订阅额度' },
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

const toUnixSeconds = (value) => {
  if (!value) return 0;
  if (value instanceof Date) return Math.floor(value.getTime() / 1000);
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) return 0;
  return Math.floor(parsed / 1000);
};

const trimValue = (value) => {
  if (value === undefined || value === null) return '';
  return String(value).trim();
};

const buildParams = (values = {}) => {
  const dateRange = Array.isArray(values.dateRange)
    ? values.dateRange
    : initialDateRange();
  const params = {
    start_timestamp: toUnixSeconds(dateRange[0]),
    end_timestamp: toUnixSeconds(dateRange[1]),
    model_name: trimValue(values.model_name),
    username: trimValue(values.username),
    user_id: Number(values.user_id || 0),
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

const formatPercent = (value) => `${(Number(value || 0) * 100).toFixed(1)}%`;

const alertColor = (severity) =>
  severity === 'critical' ? 'red' : severity === 'warning' ? 'orange' : 'green';

const formatAlertValue = (alert, field) => {
  const value = Number(alert?.[field] || 0);
  if (String(alert?.key || '').includes('rate')) {
    return `${value.toFixed(1)}%`;
  }
  return formatCount(value);
};

const BillingPage = () => {
  const { t } = useTranslation();
  const [formApi, setFormApi] = useState(null);
  const [summary, setSummary] = useState({
    items: [],
    consume_quota: 0,
    refund_quota: 0,
    net_quota: 0,
    request_count: 0,
    refund_count: 0,
    total_tokens: 0,
  });
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [statements, setStatements] = useState({ items: [], total: 0 });
  const [statementsLoading, setStatementsLoading] = useState(false);
  const [generatingStatements, setGeneratingStatements] = useState(false);
  const [alertMetrics, setAlertMetrics] = useState({
    alerts: [],
    refund_rate: 0,
    task_failure_rate: 0,
    pending_task_count: 0,
    timed_out_task_count: 0,
    worker_lag_seconds: 0,
    upstream_error_count: 0,
    insufficient_balance_count: 0,
  });
  const [alertsLoading, setAlertsLoading] = useState(false);

  const formInitValues = useMemo(
    () => ({
      dateRange: initialDateRange(),
      model_name: DEFAULT_MODEL_FILTER,
      limit: DEFAULT_LIMIT,
      logType: 0,
      billing_source: '',
      task_id: '',
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
        const params = buildParams(values || getCurrentValues());
        const res = await API.get('/api/billing/summary', { params });
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
        const params = {
          ...buildParams(values || getCurrentValues()),
          p: 1,
          page_size: 100,
        };
        const res = await API.get('/api/billing/statements', { params });
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

  const loadAlerts = useCallback(
    async (values) => {
      setAlertsLoading(true);
      try {
        const params = buildParams(values || getCurrentValues());
        const res = await API.get('/api/billing/alerts', { params });
        if (res.data.success) {
          const data = res.data.data || {};
          setAlertMetrics({
            alerts: Array.isArray(data.alerts) ? data.alerts : [],
            refund_rate: data.refund_rate || 0,
            task_failure_rate: data.task_failure_rate || 0,
            pending_task_count: data.pending_task_count || 0,
            timed_out_task_count: data.timed_out_task_count || 0,
            worker_lag_seconds: data.worker_lag_seconds || 0,
            upstream_error_count: data.upstream_error_count || 0,
            insufficient_balance_count: data.insufficient_balance_count || 0,
          });
        } else {
          showError(res.data.message || t('账单告警查询失败'));
        }
      } catch (error) {
        showError(error);
      } finally {
        setAlertsLoading(false);
      }
    },
    [getCurrentValues, t],
  );

  const handleSearch = useCallback(
    async (values) => {
      await Promise.all([
        loadSummary(values),
        loadStatements(values),
        loadAlerts(values),
      ]);
    },
    [loadSummary, loadStatements, loadAlerts],
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

  const generateBillingStatements = async () => {
    setGeneratingStatements(true);
    try {
      const values = getCurrentValues();
      const params = buildParams(values);
      const res = await API.post('/api/billing/statements/generate', null, {
        params,
      });
      if (res.data.success) {
        const count = res.data.data?.generated_count || 0;
        showSuccess(t('已生成 {{count}} 条月结快照', { count }));
        await loadStatements(values);
      } else {
        showError(res.data.message || t('月结快照生成失败'));
      }
    } catch (error) {
      showError(error);
    } finally {
      setGeneratingStatements(false);
    }
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
      const res = await API.get('/api/billing/export', {
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
      link.download = `billing-${Date.now()}.csv`;
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
      title: t('消费扣费'),
      value: renderQuota(summary.consume_quota, 6),
      desc: t('消费日志累计扣费'),
    },
    {
      title: t('退款返还'),
      value: renderQuota(summary.refund_quota, 6),
      desc: t('失败或回滚任务返还额度'),
    },
    {
      title: t('净扣费'),
      value: renderQuota(summary.net_quota, 6),
      desc: t('消费扣费减去退款返还'),
    },
    {
      title: t('调用次数'),
      value: formatCount(summary.request_count),
      desc: t('消费流水条数'),
    },
    {
      title: t('退款次数'),
      value: formatCount(summary.refund_count),
      desc: t('退款流水条数'),
    },
    {
      title: t('Token 合计'),
      value: formatCount(summary.total_tokens),
      desc: t('同步模型按 token 统计，视频任务可能为 0'),
    },
  ];

  const columns = [
    {
      title: t('用户 ID'),
      dataIndex: 'user_id',
      key: 'user_id',
      width: 100,
      render: (value) => <Text copyable>{value}</Text>,
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      key: 'username',
      width: 160,
      render: (value) => value || '-',
    },
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
    {
      title: t('退款次数'),
      dataIndex: 'refund_count',
      key: 'refund_count',
      width: 110,
      render: (value) => formatCount(value),
    },
    {
      title: t('Token 合计'),
      dataIndex: 'total_tokens',
      key: 'total_tokens',
      width: 130,
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
      title: t('用户 ID'),
      dataIndex: 'user_id',
      key: 'user_id',
      width: 100,
      render: (value) => <Text copyable>{value}</Text>,
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      key: 'username',
      width: 140,
      render: (value) => value || '-',
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
      title: t('调用次数'),
      dataIndex: 'request_count',
      key: 'request_count',
      width: 110,
      render: (value) => formatCount(value),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (value) => <Tag>{value || 'closed'}</Tag>,
    },
  ];

  const alertStats = [
    {
      title: t('退款率'),
      value: formatPercent(alertMetrics.refund_rate),
      desc: t('退款流水数 / 消费流水数'),
      severity: alertMetrics.refund_rate >= 0.2 ? 'warning' : 'ok',
    },
    {
      title: t('任务失败率'),
      value: formatPercent(alertMetrics.task_failure_rate),
      desc: t('失败任务 / 已完成任务'),
      severity: alertMetrics.task_failure_rate >= 0.3 ? 'warning' : 'ok',
    },
    {
      title: t('待处理任务'),
      value: formatCount(alertMetrics.pending_task_count),
      desc: t('尚未成功或失败的异步任务'),
      severity: alertMetrics.pending_task_count > 0 ? 'warning' : 'ok',
    },
    {
      title: t('超时任务'),
      value: formatCount(alertMetrics.timed_out_task_count),
      desc: t('超过阈值仍未完成'),
      severity: alertMetrics.timed_out_task_count > 0 ? 'critical' : 'ok',
    },
    {
      title: t('Worker 滞后'),
      value: `${formatCount(alertMetrics.worker_lag_seconds)}s`,
      desc: t('最旧待处理任务距上次更新'),
      severity: alertMetrics.worker_lag_seconds >= 900 ? 'critical' : 'ok',
    },
    {
      title: t('上游/额度信号'),
      value: formatCount(
        alertMetrics.upstream_error_count +
          alertMetrics.insufficient_balance_count,
      ),
      desc: t('上游错误与余额不足信号'),
      severity:
        alertMetrics.upstream_error_count +
          alertMetrics.insufficient_balance_count >
        0
          ? 'warning'
          : 'ok',
    },
  ];

  const searchArea = (
    <Form
      initValues={formInitValues}
      getFormApi={setFormApi}
      onSubmit={handleSearch}
      allowEmpty
      autoComplete='off'
      layout='vertical'
      trigger='change'
    >
      <div className='flex flex-col gap-2'>
        <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-2'>
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
            field='username'
            prefix={<IconSearch />}
            placeholder={t('用户名')}
            showClear
            pure
            size='small'
          />
          <Form.InputNumber
            field='user_id'
            prefix={<IconSearch />}
            placeholder={t('用户 ID')}
            min={1}
            pure
            size='small'
          />
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
          <Form.InputNumber
            field='limit'
            placeholder={t('返回条数')}
            min={1}
            max={10000}
            pure
            size='small'
          />
        </div>
        <div className='flex flex-col sm:flex-row justify-between gap-2'>
          <div className='flex flex-col sm:flex-row gap-2 sm:items-center'>
            <Form.Select
              field='logType'
              className='w-full sm:w-[140px]'
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
            <Text type='secondary' size='small'>
              {t(
                '默认筛选 Seedance 2.0 系列；任务 ID 可精确对账；导出充值时会自动忽略模型、渠道、分组、资金来源和任务筛选。',
              )}
            </Text>
          </div>
          <div className='flex gap-2 justify-end'>
            <Button
              type='tertiary'
              htmlType='submit'
              icon={<IconSearch />}
              loading={loading}
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
            <Button
              type='primary'
              icon={<IconDownload />}
              loading={exporting}
              onClick={downloadBillingCsv}
              size='small'
            >
              {t('导出流水')}
            </Button>
            <Button
              type='primary'
              theme='solid'
              loading={generatingStatements}
              onClick={generateBillingStatements}
              size='small'
            >
              {t('生成快照')}
            </Button>
          </div>
        </div>
      </div>
    </Form>
  );

  const statsArea = (
    <div className='flex flex-col gap-3'>
      <div>
        <Title heading={5} className='!mb-1'>
          {t('账单管理')}
        </Title>
        <Text type='secondary'>
          {t(
            '按用户、模型、渠道、分组、资金来源和任务 ID 汇总消费与退款，适用于 Seedance 2.0 充值扣费对账。',
          )}
        </Text>
      </div>
      <div className='grid grid-cols-1 md:grid-cols-3 xl:grid-cols-6 gap-2'>
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
    </div>
  );

  return (
    <div className='mt-[60px] px-2'>
      <CardPro type='type2' statsArea={statsArea} searchArea={searchArea} t={t}>
        <CardTable
          columns={columns}
          dataSource={(summary.items || []).map((item, index) => ({
            ...item,
            key: `${item.user_id}-${item.model_name}-${item.channel_id}-${item.group}-${item.billing_source}-${index}`,
          }))}
          rowKey='key'
          loading={loading}
          scroll={{ x: 'max-content' }}
          size='small'
          className='rounded-xl overflow-hidden'
          empty={<Empty description={t('暂无账单数据')} />}
          pagination={false}
        />
      </CardPro>
      <Card className='!rounded-xl mt-3' bordered>
        <div className='flex flex-col md:flex-row md:items-center justify-between gap-2 mb-3'>
          <div>
            <Title heading={6} className='!mb-1'>
              {t('生产监控与告警')}
            </Title>
            <Text type='secondary'>
              {t(
                '基于当前筛选窗口统计退款、失败、超时、余额不足和上游错误，帮助定位 Seedance 2.0 计费链路异常。',
              )}
            </Text>
          </div>
          <Button
            type='tertiary'
            icon={<IconRefresh />}
            loading={alertsLoading}
            onClick={() => loadAlerts(getCurrentValues())}
            size='small'
          >
            {t('刷新告警')}
          </Button>
        </div>
        <div className='grid grid-cols-1 md:grid-cols-3 xl:grid-cols-6 gap-2 mb-3'>
          {alertStats.map((item) => (
            <Card key={item.title} className='!rounded-xl' bordered>
              <div className='flex flex-col gap-1'>
                <div className='flex items-center justify-between gap-2'>
                  <Text type='secondary' size='small'>
                    {item.title}
                  </Text>
                  <Tag color={alertColor(item.severity)}>
                    {item.severity === 'ok' ? t('正常') : t('关注')}
                  </Tag>
                </div>
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
        {alertMetrics.alerts.length > 0 ? (
          <div className='flex flex-col gap-2'>
            {alertMetrics.alerts.map((alert) => (
              <div
                key={alert.key}
                className='flex flex-col md:flex-row md:items-start gap-2 justify-between border border-solid border-[var(--semi-color-border)] rounded-lg p-3'
              >
                <div className='flex flex-col gap-1'>
                  <div className='flex items-center gap-2'>
                    <Tag color={alertColor(alert.severity)}>
                      {alert.severity === 'critical' ? t('严重') : t('警告')}
                    </Tag>
                    <Text strong>{t(alert.message)}</Text>
                  </div>
                  <Text type='secondary'>{t(alert.recommendation)}</Text>
                </div>
                <Text type='tertiary' size='small'>
                  {t('当前值')}: {formatAlertValue(alert, 'value')}
                  {alert.threshold > 0
                    ? ` / ${t('阈值')}: ${formatAlertValue(alert, 'threshold')}`
                    : ''}
                </Text>
              </div>
            ))}
          </div>
        ) : (
          <Empty description={t('当前筛选窗口暂无告警')} />
        )}
      </Card>
      <Card className='!rounded-xl mt-3' bordered>
        <div className='flex flex-col md:flex-row md:items-center justify-between gap-2 mb-3'>
          <div>
            <Title heading={6} className='!mb-1'>
              {t('月结快照')}
            </Title>
            <Text type='secondary'>
              {t(
                '将当前筛选条件固化为账期快照，适合月底财务对账和客户账单留档。',
              )}
            </Text>
          </div>
          <div className='flex gap-2 justify-end'>
            <Button
              type='tertiary'
              icon={<IconRefresh />}
              loading={statementsLoading}
              onClick={() => loadStatements(getCurrentValues())}
              size='small'
            >
              {t('刷新快照')}
            </Button>
            <Button
              type='primary'
              loading={generatingStatements}
              onClick={generateBillingStatements}
              size='small'
            >
              {t('生成当前账期快照')}
            </Button>
          </div>
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
        <Text type='tertiary' size='small'>
          {t('当前筛选下共有 {{total}} 条快照记录', {
            total: statements.total || 0,
          })}
        </Text>
      </Card>
    </div>
  );
};

export default BillingPage;
