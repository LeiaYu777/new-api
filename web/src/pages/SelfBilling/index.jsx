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

import React, { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation } from 'react-router-dom';
import SelfBillingCard from '../../components/topup/SelfBillingCard';

const SelfBilling = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const initialFilters = useMemo(() => {
    const params = new URLSearchParams(location.search);
    return {
      task_id: params.get('task_id') || '',
      model_name: params.get('model_name') || '',
      billing_source: params.get('billing_source') || '',
      channel: params.get('channel') || '',
      group: params.get('group') || '',
      limit: params.get('limit') || '',
      type: params.get('type') || params.get('logType') || '',
      start_timestamp:
        params.get('start_timestamp') || params.get('start') || '',
      end_timestamp: params.get('end_timestamp') || params.get('end') || '',
    };
  }, [location.search]);

  return (
    <div className='mt-[60px] px-2'>
      <SelfBillingCard t={t} initialFilters={initialFilters} />
    </div>
  );
};

export default SelfBilling;
