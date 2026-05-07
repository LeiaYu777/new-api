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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Divider,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { ExternalLink, Mail, ShieldCheck } from 'lucide-react';
import { API, showError } from '../../helpers';

const { Title, Text, Paragraph } = Typography;

const fallbackOpenSourceInfo = {
  project: 'Modified New API',
  upstream_project: 'New API',
  upstream_maintainer: 'QuantumNous',
  upstream_license: 'GNU Affero General Public License v3.0',
  based_on: 'One API, MIT License',
  source_code_url: '',
  license_url: 'https://www.gnu.org/licenses/agpl-3.0.html',
  modifier_name: '',
  modified_version: '',
  modified_date: '',
  modification_summary:
    'Lean deployment changes, configuration changes, and service customization.',
  legal_contact_email: '',
  notice:
    'This service is a modified version of New API. The corresponding source code is available to users of this service under AGPLv3.',
};

const displayValue = (value, fallback = '未配置 / Not configured') => {
  if (typeof value !== 'string') return fallback;
  const trimmed = value.trim();
  return trimmed || fallback;
};

const OpenSource = () => {
  const [info, setInfo] = useState(fallbackOpenSourceInfo);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const loadOpenSourceInfo = async () => {
      try {
        const res = await API.get('/api/about/open-source');
        setInfo({ ...fallbackOpenSourceInfo, ...(res.data || {}) });
      } catch (error) {
        showError('开源许可证信息加载失败，请稍后重试');
      } finally {
        setLoading(false);
      }
    };

    loadOpenSourceInfo();
  }, []);

  const sourceCodeURL = displayValue(
    info.source_code_url,
    'contact administrator',
  );
  const legalContactEmail = displayValue(
    info.legal_contact_email,
    'contact administrator',
  );
  const canOpenSourceURL = Boolean(info.source_code_url);
  const canEmail = Boolean(info.legal_contact_email);

  const noticeRows = useMemo(
    () => [
      ['原项目 / Original project', displayValue(info.upstream_project)],
      [
        '原项目维护方 / Original maintainer',
        displayValue(info.upstream_maintainer),
      ],
      ['原项目许可证 / Original license', displayValue(info.upstream_license)],
      ['原项目基于 / Based on', displayValue(info.based_on)],
      ['修改方 / Modified by', displayValue(info.modifier_name)],
      ['修改版本 / Modified version', displayValue(info.modified_version)],
      ['修改日期 / Modified date', displayValue(info.modified_date)],
      ['主要修改内容 / Summary', displayValue(info.modification_summary)],
    ],
    [info],
  );

  return (
    <div className='mt-[60px] px-3 py-6 max-w-5xl mx-auto'>
      <Spin spinning={loading}>
        <Card className='!rounded-2xl shadow-sm border-0'>
          <div className='flex flex-col gap-4'>
            <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-3'>
              <div>
                <div className='flex items-center gap-2 mb-2'>
                  <ShieldCheck size={22} color='var(--semi-color-success)' />
                  <Tag color='green'>AGPLv3</Tag>
                </div>
                <Title heading={2} className='!mb-2'>
                  开源许可证与源码获取
                </Title>
                <Text type='tertiary'>
                  Open Source License and Source Code Access
                </Text>
              </div>
              <div className='flex flex-wrap gap-2'>
                <Button
                  theme='solid'
                  type='primary'
                  icon={<ExternalLink size={16} />}
                  disabled={!canOpenSourceURL}
                  onClick={() => window.open(info.source_code_url, '_blank')}
                >
                  打开修改版源码
                </Button>
                <Button
                  theme='outline'
                  icon={<ExternalLink size={16} />}
                  onClick={() => window.open(info.license_url, '_blank')}
                >
                  查看 AGPLv3
                </Button>
              </div>
            </div>

            <Divider margin='16px' />

            <section className='space-y-4'>
              <Title heading={4}>中文说明</Title>
              <Paragraph>本系统基于 New API 修改开发。</Paragraph>
              <Paragraph>
                New API 是由 QuantumNous 维护的开源项目，项目使用 GNU Affero
                General Public License v3.0（AGPLv3）授权。New API 是基于 One
                API（MIT License）开发的开源项目。
              </Paragraph>
              <Paragraph>
                本系统为 New API 的修改版本。根据 AGPLv3
                的要求，所有通过网络与本系统交互的用户，均可以获取本系统正在运行版本的对应源码。
              </Paragraph>
              <div className='rounded-xl border border-semi-color-border bg-semi-color-fill-0 p-4'>
                <Text strong>修改版源码获取地址：</Text>{' '}
                {canOpenSourceURL ? (
                  <a
                    href={info.source_code_url}
                    target='_blank'
                    rel='noopener noreferrer'
                    className='!text-semi-color-primary break-all'
                  >
                    {sourceCodeURL}
                  </a>
                ) : (
                  <Text type='warning'>{sourceCodeURL}</Text>
                )}
              </div>
              <Paragraph>
                如果以上地址暂时无法访问，请通过以下方式联系管理员获取源码：{' '}
                {canEmail ? (
                  <a
                    href={`mailto:${info.legal_contact_email}`}
                    className='!text-semi-color-primary break-all'
                  >
                    {legalContactEmail}
                  </a>
                ) : (
                  <Text type='warning'>{legalContactEmail}</Text>
                )}
              </Paragraph>
              <Paragraph>
                源码范围说明：源码获取地址提供的是本系统修改版程序的对应源码、构建脚本、部署脚本、Dockerfile、前端构建配置以及运行该程序所需的非敏感配置示例。该源码不包含客户数据、账单数据、用户隐私数据、API
                Key、数据库密码、服务器密钥、生产环境 .env 文件或其他敏感信息。
              </Paragraph>
              <Paragraph>
                许可证说明：本系统修改版整体按照 GNU Affero General Public
                License v3.0（AGPLv3）提供。你可以在 AGPLv3
                条款允许的范围内使用、复制、修改和再分发本系统源码。
              </Paragraph>
              <Paragraph>
                无担保声明：本系统按“现状”提供，不提供任何明示或暗示担保，除非双方另有书面约定。
              </Paragraph>
            </section>

            <Divider margin='16px' />

            <section className='space-y-4'>
              <Title heading={4}>English Notice</Title>
              <Paragraph>
                This service is a modified version of New API.
              </Paragraph>
              <Paragraph>
                New API is an open-source project maintained by QuantumNous and
                licensed under the GNU Affero General Public License v3.0
                (AGPLv3). New API is developed based on One API, which is
                licensed under the MIT License.
              </Paragraph>
              <Paragraph>
                This service is a modified version of New API. Under the AGPLv3,
                all users who interact with this service remotely through a
                computer network are offered an opportunity to receive the
                Corresponding Source of the version running on this server.
              </Paragraph>
              <Paragraph>
                Modified source code:{' '}
                {canOpenSourceURL ? (
                  <a
                    href={info.source_code_url}
                    target='_blank'
                    rel='noopener noreferrer'
                    className='!text-semi-color-primary break-all'
                  >
                    {sourceCodeURL}
                  </a>
                ) : (
                  <Text type='warning'>{sourceCodeURL}</Text>
                )}
              </Paragraph>
              <Paragraph>
                If the source code URL is unavailable, please contact the
                administrator:{' '}
                {canEmail ? (
                  <a
                    href={`mailto:${info.legal_contact_email}`}
                    className='!text-semi-color-primary break-all'
                  >
                    {legalContactEmail}
                  </a>
                ) : (
                  <Text type='warning'>{legalContactEmail}</Text>
                )}
              </Paragraph>
              <Paragraph>
                Scope of source code: The source code package or repository
                includes the corresponding source code of this modified version,
                build scripts, deployment scripts, Dockerfile, frontend build
                configuration, and non-sensitive example configuration required
                to build and run the program. It does not include customer data,
                billing data, private user data, API keys, database passwords,
                server credentials, production .env files, or other secrets.
              </Paragraph>
              <Paragraph>
                License: This modified version is provided under the GNU Affero
                General Public License v3.0 (AGPLv3).
              </Paragraph>
              <Paragraph>
                No warranty: This service is provided “as is”, without warranty
                of any kind, unless otherwise agreed in writing.
              </Paragraph>
            </section>

            <Divider margin='16px' />

            <section>
              <Title heading={4}>
                归属与修改说明 / Attribution and Modification Notice
              </Title>
              <div className='overflow-x-auto rounded-xl border border-semi-color-border'>
                <table className='w-full text-sm'>
                  <tbody>
                    {noticeRows.map(([label, value]) => (
                      <tr
                        key={label}
                        className='border-b border-semi-color-border last:border-b-0'
                      >
                        <th className='text-left align-top p-3 w-64 bg-semi-color-fill-0 font-medium'>
                          {label}
                        </th>
                        <td className='p-3 break-words'>{value}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>

            <div className='rounded-xl bg-semi-color-info-light-default p-4 flex gap-3 items-start'>
              <Mail size={18} className='mt-1 flex-shrink-0' />
              <Text>
                本页面仅用于展示开源许可证、原项目归属、修改说明与源码获取方式；不会展示
                API Key、数据库连接、客户数据、账单明细或服务器密钥。
              </Text>
            </div>
          </div>
        </Card>
      </Spin>
    </div>
  );
};

export default OpenSource;
