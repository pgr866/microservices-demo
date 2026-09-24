import demo_pb2
from email_server import template


def make_item(product_id, quantity, units, nanos):
  return demo_pb2.OrderItem(
    item=demo_pb2.CartItem(product_id=product_id, quantity=quantity),
    cost=demo_pb2.Money(currency_code='USD', units=units, nanos=nanos),
  )


def build_order(items):
  return demo_pb2.OrderResult(
    order_id='123',
    shipping_tracking_id='TRACK-1',
    shipping_cost=demo_pb2.Money(currency_code='USD', units=5, nanos=990000000),
    shipping_address=demo_pb2.Address(
      street_address='1600 Amphitheatre Pkwy',
      city='Mountain View',
      country='USA',
      zip_code=94043,
    ),
    items=items,
  )


def test_renders_order_and_shipping_details():
  order = build_order([make_item('OLJCESPC7Z', 2, 19, 990000000)])
  html = template.render(order=order)

  assert '123' in html
  assert 'TRACK-1' in html
  assert '1600 Amphitheatre Pkwy' in html
  assert 'Mountain View' in html


def test_renders_item_quantity_and_price():
  order = build_order([make_item('OLJCESPC7Z', 2, 19, 990000000)])
  html = template.render(order=order)

  assert 'OLJCESPC7Z' in html
  assert '19.99 USD' in html


def test_renders_multiple_items():
  order = build_order([
    make_item('OLJCESPC7Z', 2, 19, 990000000),
    make_item('66VCHSJNUP', 1, 18, 990000000),
  ])
  html = template.render(order=order)

  assert 'OLJCESPC7Z' in html
  assert '66VCHSJNUP' in html
