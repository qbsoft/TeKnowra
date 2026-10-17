"""Trusted pure computation: stdin JSON -> stdout JSON, no file/network/env reads.

Business rule provenance is supplied by the review flow; arithmetic does not
authenticate the retrieved rule or the model's extraction of contract facts.
"""
import json
import re
import sys
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP


def main():
    try:
        if len(sys.argv) != 1:
            raise ValueError('arguments not accepted')
        source = sys.stdin.read(65537)
        if len(source.encode('utf-8')) > 65536:
            raise ValueError('input too large')
        data = json.loads(source)
        if not isinstance(data, dict) or set(data) != {'amount', 'prepayment_percent', 'review_threshold_percent', 'rule_reference'}:
            raise ValueError('unexpected fields')
        if not isinstance(data['amount'], str) or not re.fullmatch(r'[0-9]{1,12}(\.[0-9]{1,2})?', data['amount']):
            raise ValueError('invalid amount')
        for key in ('prepayment_percent', 'review_threshold_percent'):
            if isinstance(data[key], bool) or not isinstance(data[key], (int, float)):
                raise ValueError('percentage must be numeric')
        amount = Decimal(data['amount'])
        percent = Decimal(str(data['prepayment_percent']))
        threshold = Decimal(str(data['review_threshold_percent']))
        reference = data['rule_reference']
        if not percent.is_finite() or not threshold.is_finite() or not 0 <= percent <= 100 or not 0 <= threshold <= 100:
            raise ValueError('invalid percentage')
        if not isinstance(reference, str) or not reference.strip() or len(reference) > 512 or any(ord(c) < 32 for c in reference):
            raise ValueError('rule reference required')
        value = (amount * percent / 100).quantize(Decimal('0.01'), rounding=ROUND_HALF_UP)
        print(json.dumps({'prepayment_amount': format(value, '.2f'),
                          'threshold_exceeded': percent > threshold,
                          'review_threshold_percent': str(threshold),
                          'rule_reference': reference, 'read_only': True}, ensure_ascii=True))
    except (KeyError, TypeError, ValueError, InvalidOperation):
        print('invalid structured calculation input', file=sys.stderr)
        return 2
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
