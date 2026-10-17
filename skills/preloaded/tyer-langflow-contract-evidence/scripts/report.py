"""Read-only payment calculation; no network, credential or business write access."""
import json
import sys
from decimal import Decimal, InvalidOperation, ROUND_HALF_UP

def main():
    try:
        source = sys.stdin.read(65537)
        if len(source) > 65536:
            raise ValueError('input too large')
        data = json.loads(source)
        amount = Decimal(str(data['amount']))
        percent = Decimal(str(data['prepayment_percent']))
        if not amount.is_finite() or not percent.is_finite() or not 0 <= amount <= Decimal('1e15') or not 0 <= percent <= 100:
            raise ValueError('invalid amount or percentage')
        result = (amount * percent / 100).quantize(Decimal('0.01'), rounding=ROUND_HALF_UP)
        print(json.dumps({'rule':'TK-CONTRACT-PREPAYMENT-30', 'prepayment_amount':format(result, '.2f'), 'status':'需要复核' if percent > 30 else '未触发本条规则', 'read_only':True}, ensure_ascii=False))
    except (KeyError, TypeError, ValueError, InvalidOperation):
        print('invalid payment calculation input', file=sys.stderr)
        return 2
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
