function getResourceTypeName(resourceType: string) {
  const resourceTypeMap = {
    QCLOUDCVM: '腾讯云虚拟机',
    IDCPM: 'IDC物理机',
    OTHERS: '其他',
  };
  return resourceTypeMap[resourceType];
}

function getReturnPlanName(returnPlan: string, resourceType: string) {
  if (returnPlan === 'IMMEDIATE') {
    return resourceType === 'IDCPM' ? '立即销毁（隔离2小时）' : '立即销毁';
  }
  if (returnPlan === 'DELAY') {
    let label = '延迟销毁';
    if (resourceType === 'IDCPM') {
      label += '（隔离1天）';
    } else if (resourceType === 'QCLOUDCVM') {
      label += '（隔离15天）';
    }
    return label;
  }
  return '';
}

export { getResourceTypeName, getReturnPlanName };
